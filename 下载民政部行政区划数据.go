// 下载民政部行政区划数据

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	geojson "github.com/paulmach/go.geojson"
)

type (
	// XZQXX 行政区信息
	XZQXX struct {
		Diji       string `json:"diji"`
		QuHuaDaiMa string `json:"quHuaDaiMa"`
		Quhao      string `json:"quhao"`
		Shengji    string `json:"shengji"`
		Xianji     string `json:"xianji"`
	}

	XZQXX2 struct {
		Level    int       `json:"level"`
		Abbr     string    `json:"Abbr"`
		Type     string    `json:"Type"`
		Position []float64 `json:"Position"`
		EnName   string    `json:"EnName"`
		EnAbbr   string    `json:"EnAbbr"`
		Bound    []float64 `json:"bound"`
	}
)

var (
	// 所有县的基本信息
	xianInfo map[string][]string
	// 所有县的信息
	xianInfo2 map[string]XZQXX2
	// 所有县geojson要素
	allXian map[string][]geojson.Feature
	// 所有地级市(包括直辖市)要素
	allDiqu map[string][]geojson.Feature
)

// DownloadBianjie 下载边界数据
func DownloadBianjie(qhdm string) ([]byte, error) {
	req, err := http.NewRequest("GET", "http://xzqh.mca.gov.cn/data/"+qhdm+".json", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", "浏览器调试可获取")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", "http://xzqh.mca.gov.cn/defaultQuery?")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:73.0) Gecko/20100101 Firefox/73.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("状态码:%d", resp.StatusCode)
	}
	return ioutil.ReadAll(resp.Body)
}

// TopojsonToGeojson 转换到geojson
func TopojsonToGeojson(ci XZQXX, topojsonpath, geojsonpath string) error {
	_, err := os.Stat(geojsonpath)
	if err != nil {
		// ogr2ogr -f "GeoJSON" 输出.geojson 输入.topojson [可选 图层名]
		cmd := exec.Command("C:/QGIS/QGIS 3.10/bin/ogr2ogr.exe", "-f", "GeoJSON", geojsonpath, topojsonpath)
		cmd.Env = append(cmd.Env, "GDAL_DATA=C:/QGIS/QGIS 3.10/share/gdal")
		cmd.Stdout = os.Stdout

		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("运行错误:%v[%v]", err, cmd.Args)
		}
	}
	// 读取后进行清理
	geojsondata, err := ioutil.ReadFile(geojsonpath)
	if err != nil {
		return fmt.Errorf("读取geojson出错:%v", err)
	}
	fc, err := geojson.UnmarshalFeatureCollection(geojsondata)
	err = json.Unmarshal(geojsondata, &fc)
	if err != nil {
		return fmt.Errorf("解析出错:%v", err)
	}

	// 行政区划代码长度，直辖市中间2位是00
	prefixlen := 4
	if ci.QuHuaDaiMa[2:6] == "0000" {
		prefixlen = 2
	}
	for i := 0; i < len(fc.Features); i++ {
		f := fc.Features[i]
		quhuadaima, err := f.PropertyString("QUHUADAIMA")
		// 跳过外部区域
		if err != nil ||
			len(quhuadaima) != 6 || quhuadaima == "fanwei" ||
			quhuadaima[0:prefixlen] != ci.QuHuaDaiMa[0:prefixlen] {
			continue
		}
		// 判断是县级还是地市
		if quhuadaima == ci.QuHuaDaiMa {
			_, ok := allDiqu[ci.QuHuaDaiMa]
			if !ok {
				allDiqu[quhuadaima] = []geojson.Feature{*f}
			} else {
				// 一个地区可能有多个多边形，后面再进行合并
				allDiqu[quhuadaima] = append(allDiqu[quhuadaima], *f)
			}
		} else {
			_, ok := allXian[quhuadaima]
			if !ok {
				allXian[quhuadaima] = []geojson.Feature{*f}
			} else {
				allXian[quhuadaima] = append(allXian[quhuadaima], *f)
			}
		}
	}
	return nil
}

// geojsonprocess 转换出的 GeoJSON 数据进行处理，因为转换出来的是一个个分开的，需要将有多个部分的进行合并
func geojsonprocess(dclsj map[string][]geojson.Feature, filename string) error {
	buf := bytes.NewBuffer(nil)
	for daima, fv := range dclsj {
		polycount := len(fv)
		name, err := fv[0].PropertyString("NAME")
		if err != nil {
			log.Println("错误：", daima, err)
		}
		var feature *geojson.Feature = nil
		if polycount == 1 {
			feature = &fv[0]
		} else {
			// 将多个 多边形 合并为一个 多多边形
			polygons := make([][][][]float64, 0, polycount)
			for i := 0; i < polycount; i++ {
				if fv[i].Geometry.Polygon != nil {
					polygons = append(polygons, fv[i].Geometry.Polygon)
				}
			}
			fmpoly := geojson.NewMultiPolygonFeature(polygons...)
			filecolor, _ := fv[0].PropertyString("FillColor")
			fmpoly.SetProperty("FillColor", filecolor)
			feature = fmpoly
		}
		// 设置属性
		{
			feature.SetProperty("名称", name)
			feature.SetProperty("代码", daima)
		}
		sx, ok := xianInfo[daima]
		if ok {
			feature.SetProperty("驻地", sx[1])
			num, _ := strconv.Atoi(sx[2])
			feature.SetProperty("人口", num)
			num, _ = strconv.Atoi(sx[3])
			feature.SetProperty("面积", num)
			feature.SetProperty("区号", sx[4])
			feature.SetProperty("类型", sx[6])
		}
		sx2, ok := xianInfo2[daima]
		if ok {
			feature.SetProperty("简称", sx2.Abbr)
			feature.SetProperty("英文名", sx2.EnName)
			feature.SetProperty("英文简称", sx2.EnAbbr)
			feature.SetProperty("级别", sx2.Level)
			feature.BoundingBox = sx2.Bound
		}

		data, err := json.Marshal(feature)
		if err != nil {
			return err
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}
	return ioutil.WriteFile(filename, buf.Bytes(), os.ModePerm)
}

func main() {
	// 读取地级（包括直辖市）行政区划列表
	data, err := ioutil.ReadFile("data/地级行政区列表.json")
	if err != nil {
		log.Println(err)
		return
	}
	var qhxx []XZQXX
	err = json.Unmarshal(data, &qhxx)
	if err != nil {
		log.Println(err)
		return
	}
	// 读取县级行政区划数据
	data, err = ioutil.ReadFile("data/全国县级行政区信息.json")
	if err != nil {
		log.Println(err)
		return
	}
	xianInfo = make(map[string][]string)
	err = json.Unmarshal(data, &xianInfo)
	if err != nil {
		log.Println(err)
		return
	}
	// 读取全国行政区划信息数据
	data, err = ioutil.ReadFile("data/行政区信息.json")
	if err != nil {
		log.Println(err)
		return
	}
	xianInfo2 = make(map[string]XZQXX2)
	err = json.Unmarshal(data, &xianInfo2)
	if err != nil {
		log.Println(err)
		return
	}

	allXian = make(map[string][]geojson.Feature)
	allDiqu = make(map[string][]geojson.Feature)

	for i := 0; i < len(qhxx); i++ {
		if len(qhxx[i].QuHuaDaiMa) != 6 {
			continue
		}
		topojsonpath := "data/download/" + qhxx[i].QuHuaDaiMa + ".topojson"
		geojsonpath := "data/download/" + qhxx[i].QuHuaDaiMa + ".geojson"

		_, err := os.Stat(topojsonpath)
		if err != nil {
			data, err = DownloadBianjie(qhxx[i].QuHuaDaiMa)
			if err != nil {
				log.Println("下载出错", err, qhxx[i].QuHuaDaiMa, qhxx[i].Diji)
				continue
			}
			err := ioutil.WriteFile(topojsonpath, data, os.ModePerm)
			if err != nil {
				log.Println("下载写入", err)
				continue
			}
		} else {
			data, err = ioutil.ReadFile(topojsonpath)
		}
		log.Println("下载完成", qhxx[i].QuHuaDaiMa, qhxx[i].Diji)
		err = TopojsonToGeojson(qhxx[i], topojsonpath, geojsonpath)
		if err != nil {
			log.Println("转换出错", err)
			return
		}
		log.Println("转换完成")
	}
	err = geojsonprocess(allXian, "data/全国县级.geojson")
	if err != nil {
		log.Println("县级处理出错", err)
	}
	log.Println("县级处理完成")
	err = geojsonprocess(allDiqu, "data/全国地级.geojson")
	if err != nil {
		log.Println("地级处理出错", err)
	}
	log.Println("地级处理完成")
}
