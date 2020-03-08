// 下载民政部行政区划数据
/// +build ignoro

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


// 获取下一级的行政区代码 xosw=100000&type=1 全国县级
// http://xzqh.mca.gov.cn/getInfo?code=420800&type=2
// 响应 4县级市 5区县 {"420802":"5","420804":"5","420881":"4","420882":"4","420822":"5"}
// 获取行政区名称显示位置线
// http://xzqh.mca.gov.cn/data/420800_Line.geojson
// 获取行政区政府驻地坐标
// http://xzqh.mca.gov.cn/data/420800_Point.geojson
// 获取行政区代码
// http://xzqh.mca.gov.cn/jsp/getInfo.jsp?shengji=湖北省(鄂)&diji=荆门市&xianji=-1
// 获取行政区下一级信息
// POST http://xzqh.mca.gov.cn/selectJson
// 表单数据 shengji=湖北省(鄂)&diji=荆门市

const (
	cookiestring string = "JSESSIONID=1ED17D614EEECEA151D0A60DED220C13"
)

type (
	// XzqhXX 行政区信息
	XzqhXX struct {
		Diji       string `json:"diji"`
		QuHuaDaiMa string `json:"quHuaDaiMa"`
		Quhao      string `json:"quhao"`
		Shengji    string `json:"shengji"`
		Xianji     string `json:"xianji"`
	}

	// XzqhXX2 行政区划信息2
	XzqhXX2 struct {
		Level    int       `json:"level"`
		Abbr     string    `json:"Abbr"`
		Type     string    `json:"Type"`
		Position []float64 `json:"Position"`
		EnName   string    `json:"EnName"`
		EnAbbr   string    `json:"EnAbbr"`
		Bound    []float64 `json:"bound"`
	}
	// ZdZbXX 驻地坐标信息
	ZdZbXX struct {
		Position []float64
		Location int // 文字标注的位置
		//   1 | 2 | 3
		//  -4-+- -+-6-
		//   7 | 8 | 8
		Alignment int // 文字标注的对齐方式
		// 1     o 文字
		// 2     文 o 字  文字标注位于驻地点上方或者下方
		// 3     文字 o
	}
)

var (
	// 所有县的基本信息
	xianInfo map[string][]string
	// 所有县的信息
	xianInfo2 map[string]XzqhXX2
	// 所有县的驻地信息
	xianZDInfo map[string]ZdZbXX

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
	req.Header.Set("Cookie", cookiestring)
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

// downloadZDXX 下载政府驻地信息
func downloadZDXX(qhdm string) ([]byte, error) {
	req, err := http.NewRequest("GET", "http://xzqh.mca.gov.cn/data/"+qhdm+"_Point.geojson", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", cookiestring)
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

// convTopojsonToGeojson 转换到geojson
func convTopojsonToGeojson(ci XzqhXX, topojsonpath, geojsonpath string) error {
	_, err := os.Stat(geojsonpath)
	if err != nil {
		// GDAL 命令处理
		// ogr2ogr -f "GeoJSON" 输出.geojson 输入.topojson [可选 图层名]
		cmd := exec.Command("C:/QGIS/bin/ogr2ogr.exe", "-f", "GeoJSON", geojsonpath, topojsonpath)
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
	// err = json.Unmarshal(geojsondata, &fc)
	if err != nil {
		return fmt.Errorf("解析出错:%v", err)
	}

	// 行政区划代码长度，直辖市中间2位是00
	prefixlen := 4
	if ci.QuHuaDaiMa[2:6] == "0000" {
		prefixlen = 2
	}
	//fmt.Println("fea count = ", len(fc.Features))
	for i := 0; i < len(fc.Features); i++ {
		f := fc.Features[i]
		quhuadaima, err := f.PropertyString("QUHUADAIMA")
		// 跳过外部区域
		if err != nil ||
			len(quhuadaima) != 6 || quhuadaima == "fanwei" ||
			quhuadaima[0:prefixlen] != ci.QuHuaDaiMa[0:prefixlen] {
			//fmt.Println("跳过:", quhuadaima, f.Properties.NAME)
			continue
		}
		// 判断是县级还是地市
		if quhuadaima == ci.QuHuaDaiMa {
			_, ok := allDiqu[ci.QuHuaDaiMa]
			if !ok {
				allDiqu[quhuadaima] = []geojson.Feature{*f}
				//fmt.Println("地级:", quhuadaima, f.Properties.NAME)
			} else {
				// 一个地区可能有多个多边形，后面再进行合并
				allDiqu[quhuadaima] = append(allDiqu[quhuadaima], *f)
				//fmt.Println("地级:", quhuadaima, f.Properties.NAME, len(allDiqu[quhuadaima]))
			}
		} else {
			_, ok := allXian[quhuadaima]
			if !ok {
				allXian[quhuadaima] = []geojson.Feature{*f}
			} else {
				allXian[quhuadaima] = append(allXian[quhuadaima], *f)
				//fmt.Println("县级:", quhuadaima, f.Properties.NAME, len(allXian[quhuadaima]))
			}
		}
	}
	return nil
}

// geojsonprocess 转换出的 GeoJSON 数据进行处理，因为转换出来的是一个个分开的，需要将有多个部分的进行合并
func geojsonprocess(dclsj map[string][]geojson.Feature, filename string) error {
	buf := bytes.NewBuffer(nil)
	for qhdm, fv := range dclsj {
		polycount := len(fv)
		name, err := fv[0].PropertyString("NAME")
		if err != nil {
			log.Println("错误：", qhdm, err)
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
			feature.SetProperty("代码", qhdm)
		}
		sx, ok := xianInfo[qhdm]
		if ok {
			feature.SetProperty("驻地", sx[1])
			num, _ := strconv.Atoi(sx[2])
			feature.SetProperty("人口", num)
			num, _ = strconv.Atoi(sx[3])
			feature.SetProperty("面积", num)
			feature.SetProperty("区号", sx[4])
			feature.SetProperty("类型", sx[6])
		}
		sx2, ok := xianInfo2[qhdm]
		if ok {
			feature.SetProperty("简称", sx2.Abbr)
			feature.SetProperty("英文名", sx2.EnName)
			feature.SetProperty("英文简称", sx2.EnAbbr)
			feature.SetProperty("级别", sx2.Level)
			// feature.SetProperty("Type", sx2.Type)
			feature.BoundingBox = sx2.Bound
		}
		zdx, ok := xianZDInfo[qhdm]
		if ok {
			feature.SetProperty("驻地点", zdx.Position)
			feature.SetProperty("标注对齐", zdx.Alignment)
			feature.SetProperty("标注位置", zdx.Location)
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
	var qhxx []XzqhXX
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
	xianInfo2 = make(map[string]XzqhXX2)
	err = json.Unmarshal(data, &xianInfo2)
	if err != nil {
		log.Println(err)
		return
	}

	xianZDInfo = make(map[string]ZdZbXX)
	allXian = make(map[string][]geojson.Feature)
	allDiqu = make(map[string][]geojson.Feature)

	for i := 0; i < len(qhxx); i++ {
		if len(qhxx[i].QuHuaDaiMa) != 6 {
			continue
		}
		qhdm := qhxx[i].QuHuaDaiMa
		topojsonpath := "data/download/" + qhdm + ".topojson"
		geojsonpath := "data/download/" + qhdm + ".geojson"
		zdpointpath := "data/download/" + qhdm + ".point.geojson"

		_, err := os.Stat(topojsonpath)
		if err != nil {
			data, err = DownloadBianjie(qhdm)
			if err != nil {
				log.Println("下载出错", err, qhdm, qhxx[i].Diji)
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
		log.Println("下载完成", qhdm, qhxx[i].Diji)
		err = convTopojsonToGeojson(qhxx[i], topojsonpath, geojsonpath)
		if err != nil {
			log.Println("转换出错", err)
			return
		}
		log.Println("转换完成")
		// 下载驻地信息
		data, err := ioutil.ReadFile(zdpointpath)
		if err != nil {
			// 下载驻地信息
			data, err = downloadZDXX(qhdm)
			if err == nil {
				err = ioutil.WriteFile(zdpointpath, data, os.ModePerm)
			}
		}
		if data != nil {
			fc, err := geojson.UnmarshalFeatureCollection(data)
			if err != nil {
				fmt.Printf("解析出错:%s(%v)", qhdm, err)
			} else {
				for fidx := 0; fidx < len(fc.Features); fidx++ {
					f := fc.Features[fidx]
					qhdmxian, err := f.PropertyString("QUHUADAIMA")
					if err == nil {
						var zdx ZdZbXX
						zdx.Position = f.Geometry.Point
						zdx.Location, _ = f.PropertyInt("Location")
						zdx.Alignment, _ = f.PropertyInt("Alignment")
						xianZDInfo[qhdmxian] = zdx
					}
				}
			}
		}

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
