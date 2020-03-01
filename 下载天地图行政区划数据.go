// 下载天地图行政区划数据.go

// +build ignoro

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type (
	// ResultData 天地图接口返回数据
	ResultData struct {
		Returncode string     `json:"returncode"`
		Msg        string     `json:"msg"`
		Data       []DiQuData `json:"data"`
	}
	// DiQuData 返回数据中的一个地区的数据（省市县）
	DiQuData struct {
		Level              int     `json:"level"`
		Nameabbrevation    string  `json:"nameabbrevation"`
		Name               string  `json:"name"`
		AdminType          string  `json:"adminType"`
		CityCode           string  `json:"cityCode"`
		Lnt                float64 `json:"lnt"`
		Englishabbrevation string  `json:"englishabbrevation"`
		English            string  `json:"english"`
		Bound              string  `json:"bound"`
		Lat                float64 `json:"lat"`
		Points             []struct {
			Region string `json:"region"`
		} `json:"points"`
		Child []DiQuData `json:"child"`
	}

	// Feature 一个GeoJSON要素（多边形）
	Feature struct {
		Type       string `json:"type"`
		Name       string `json:"name"`
		ID         string `json:"id"`
		Properties struct {
			Level         int        `json:"level"`
			Abbrevation   string     `json:"简称"`
			XZLB          string     `json:"类别"`
			Position      [2]float64 `json:"Position"`
			EnName        string     `json:"英文名"`
			EnAbbrevation string     `json:"英文简称"`
			Bound         [4]float64 `json:"bound"`
		} `json:"properties"`
		Geometry struct {
			Type        string         `json:"type"`
			Coordinates [][][2]float64 `json:"coordinates"`
		} `json:"geometry"`
	}
)

// Convert 将返回的数据，转换为GeoJSON要素
func Convert(in *DiQuData) []Feature {
	var features []Feature = make([]Feature, 0, 128)
	return ConvertImpl(in, "", features)
}

// ConvertImpl 转换的实际实现
func ConvertImpl(in *DiQuData, perfix string, features []Feature) []Feature {
	var feature Feature
	feature.Name = perfix + in.Name
	feature.Type = "Feature"
	feature.ID = in.CityCode[3:]
	feature.Properties.Level = in.Level
	feature.Properties.Abbrevation = in.Nameabbrevation
	switch in.AdminType {
	case "county":
		feature.Properties.XZLB = "县"
	case "city":
		feature.Properties.XZLB = "地"
	case "province":
		feature.Properties.XZLB = "省"
	}
	feature.Properties.Position[0] = in.Lnt
	feature.Properties.Position[1] = in.Lat
	feature.Properties.EnName = in.English
	feature.Properties.EnAbbrevation = in.Englishabbrevation
	var x0, y0, x1, y1 float64
	fmt.Sscanf(in.Bound, "%f,%f,%f,%f", &x0, &y1, &x1, &y0)
	feature.Properties.Bound[0] = x0
	feature.Properties.Bound[1] = y0
	feature.Properties.Bound[2] = x1
	feature.Properties.Bound[3] = y1
	feature.Geometry.Type = "Polygon"
	ringNum := len(in.Points)
	feature.Geometry.Coordinates = make([][][2]float64, ringNum)
	for i := 0; i < ringNum; i++ {
		region := strings.Split(in.Points[i].Region, ",")
		ptNum := len(region)
		feature.Geometry.Coordinates[i] = make([][2]float64, 0, ptNum)
		for j := 0; j < ptNum; j++ {
			var x, y float64
			fmt.Sscanf(region[j], "%f %f", &x, &y)
			feature.Geometry.Coordinates[i] = append(feature.Geometry.Coordinates[i], [2]float64{x, y})
		}
	}
	features = append(features, feature)
	for i := 0; i < len(in.Child); i++ {
		features = ConvertImpl(&in.Child[i], feature.Name+"/", features)
	}
	return features
}

// 获取省级行政区划数据
func GetProvinceData(province string) ([]byte, error) {
	// 接口信息
	// http://lbs.tianditu.gov.cn/server/administrative.html
	// 超时时间：5秒
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", "http://api.tianditu.gov.cn/administrative", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Host", "api.tianditu.gov.cn")
	req.Header.Add("Cookie", "浏览器调试可获取")
	req.Header.Add("Accept-Language", "zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2")
	req.Header.Add("Referer", "https://www.tianditu.gov.cn/")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:72.0) Gecko/20100101 Firefox/72.0")
	req.Header.Add("Accept", "*/*")

	var queryparams = url.Values{}
	queryparams.Add("postStr", fmt.Sprintf(`{"searchWord":"%s","searchType":"1","needSubInfo":"true","needAll":"true","needPolygon":"true","needPre":"false"}`, province))
	queryparams.Add("tk", "天地图官网获取")
	req.URL.RawQuery = queryparams.Encode()
	fmt.Println(req.URL)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

func main() {
	provinces := []string{
		"北京", "天津", "上海", "重庆",
		"河北", "山西", "辽宁", "吉林", "黑龙江", "江苏", "浙江", "安徽", "福建", "江西", "山东", "河南",
		"湖北", "湖南", "广东", "海南", "四川", "贵州", "云南", "陕西", "甘肃", "青海", "台湾", "内蒙古",
		"广西", "西藏", "宁夏", "新疆", "香港", "澳门",
	}
	var features []Feature = make([]Feature, 0, 4096)
	// 逐个省获取
	for _, province := range provinces {
		data, err := GetProvinceData(province)
		if err != nil {
			fmt.Println(province, err.Error())
		}
		var r ResultData
		err = json.Unmarshal(data, &r)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		fmt.Println(r.Returncode, r.Msg)
		for i := 0; i < len(r.Data); i++ {
			features = ConvertImpl(&r.Data[i], "", features)
		}
	}

	outdata, err := /*json.Marshal(&features) /*/ json.MarshalIndent(&features, "", "  ")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	// fmt.Println(string(outdata))
	err = ioutil.WriteFile("xzqh3.json", outdata, os.ModePerm)
	if err != nil {
		fmt.Println(err.Error())
	}
}
