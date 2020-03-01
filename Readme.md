# 抓取行政区划数据

[TOC]

## 天地图接口

天地图官网都有相关介绍，这里只是简单的搬运一下。
接口说明地址：[http://lbs.tianditu.gov.cn/server/administrative.html](http://lbs.tianditu.gov.cn/server/administrative.html)

### 接口信息

天地图行政区划API是一类简单的HTTP/HTTPS接口，提供由行政区划地名、行政区划编码查询中心点、轮廓、所属上级行政区划的功能。

请求：

```bash
http://api.tianditu.gov.cn/administrative?postStr={"searchWord":"北京","searchType":"1","needSubInfo":"false","needAll":"false","needPolygon":"true","needPre":"true"}&tk=您的密钥
```

返回：

```json
{
    "msg": "ok",
    "data": [{
        "lnt": 116.40100299989,
        "adminType": "province",
        "englishabbrevation": "BeiJing",
        "nameabbrevation": "北京",
        "level": 11,
        "cityCode": "156110000",
        "bound": "115.422051,40.978643,117.383319,39.455766",
        "name": "北京市",
        "english": "BeiJing Shi",
        "lat": 39.90311700025,
        "points": [{
            "region": "117.383 40.226,117.203 40.081,116.775 40.034,116.78 39.888,116.92 39.834,116.9 39.687,116.806 39.615,116.563 39.619,116.328 39.456,116.235 39.563,115.918 39.596,115.815 39.508,115.566 39.591,115.48 39.74,115.517 39.898,115.422 39.983,115.589 40.115,115.829 40.144,115.956 40.268,115.766 40.442,115.902 40.616,116.067 40.611,116.213 40.741,116.451 40.797,116.449 40.979,116.672 40.97,116.959 40.708,117.283 40.659,117.223 40.386,117.383 40.226"
        }],
        "parents": {
            "country": {
                "adminType": "country",
                "cityCode": "156000000",
                "name": "中华人民共和国"
            }
        }
    }],
    "returncode": "100",
    "dataversion": "20180719",
    "dataInsertMess": "数据库已存在该版本，不进行导入"
}
```

### 代码

代码可见 [下载天地图行政区划数据](下载天地图行政区划数据.go)

## 民政部数据

民政部网站没有提供相关的接口，但是可以从查询网站调试获取。民政部的数据比较分散，需要从多个接口读取然后进行组合。

民政部的数据当前是更新到2018年的，且坐标是有偏移，需要进一步处理。

注：民政部的数据不包括港澳台地区详细数据。

### 获取全国县级行政区信息

请求：

```bash
curl --request GET \
  --url 'http://xzqh.mca.gov.cn/getInfo?code=100000&type=1' \
  --header 'pragma: no-cache' \
  --header 'referer: http://xzqh.mca.gov.cn/defaultQuery?' \
  --header 'user-agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:73.0) Gecko/20100101 Firefox/73.0' \
  --cookie 浏览器调试获取
```

返回：

结果中依次为：名称、驻地、人口（万）、面积（平方千米）、区号、代码、类型、省、地市

```json
{
  "110101": [
    "东城区",
    "景山街道",
    "97",
    "42",
    "010",
    "100010",
    "市辖区",
    "北京市",
    ""
  ],
  "110102": [
    "西城区",
    "金融街街道",
    "139",
    "51",
    "010",
    "100032",
    "市辖区",
    "北京市",
    ""
  ],
  ...
  "440983": [
    "信宜市",
    "东镇街道",
    "136",
    "3081",
    "0668",
    "525300",
    "县级市",
    "广东省",
    "茂名市"
  ],
```

### 全国县级行政区边界

这个接口获取的是一个 [TopoJSON](https://www.jianshu.com/p/465702337744) 的数据，可以使用 [ogr2ogr](https://gdal.org/programs/ogr2ogr.html#ogr2ogr) 程序转换为 GeoJSON 等，也可以直接使用 QGIS 打开查看。

请求：

```bash
curl --request GET \
  --url http://xzqh.mca.gov.cn/data/xian_quanguo.json \
  --header 'pragma: no-cache' \
  --header 'referer: http://xzqh.mca.gov.cn/defaultQuery?' \
  --header 'user-agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:73.0) Gecko/20100101 Firefox/73.0' \
  --cookie 浏览器调试获取
```

返回：

```json
{"type":"Topology","arcs":[[[122624,100420],[28,-20],[21,-71],[41,3],[43,-83]],[[122757,100249],[47,-24],[79,-103],[6,-13],[65,-107],[-10,-64],[-3,-7],[-27,-100],[44,-36],[5,-15],[-22    ...   {"arcs":[[9424]],"type":"Polygon","properties":{"NAME":"460300","QUHUADAIMA":"daodian","FillColor":""}},{"arcs":[[9425]],"type":"Polygon","properties":{"NAME":"460300","QUHUADAIMA":"daodian","FillColor":""}},{"arcs":[[9426]],"type":"Polygon","properties":{"NAME":"460300","QUHUADAIMA":"daodian","FillColor":""}},{"arcs":[[9427]],"type":"Polygon","properties":{"NAME":"460300","QUHUADAIMA":"daodian","FillColor":""}},{"arcs":[[9428]],"type":"Polygon","properties":{"NAME":"460300","QUHUADAIMA":"daodian","FillColor":""}},{"arcs":[[9429]],"type":"Polygon","properties":{"NAME":"460300","QUHUADAIMA":"daodian","FillColor":""}},{"arcs":[[9430]],"type":"Polygon","properties":{"NAME":"460300","QUHUADAIMA":"daodian","FillColor":""}}]}},"crs":{"type":"name","properties":{"name":"urn:ogc:def:crs:OGC:1.3:CRS84"}}}
```



### 政府驻地地理位置

这个接口用于获取某个地区的政府驻地坐标，用于在地图上显示区域关键点。

请求（url 中的文件名是行政区的代码）：

```json
curl --request GET \
  --url http://xzqh.mca.gov.cn/data/430600_Point.geojson \
  --header 'pragma: no-cache' \
  --header 'referer: http://xzqh.mca.gov.cn/defaultQuery?' \
  --header 'user-agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:73.0) Gecko/20100101 Firefox/73.0' \
  --cookie 浏览器调试获取
```

返回：

```json
{
  "type": "FeatureCollection",
  "name": "点",
  "features": [
    {
      "type": "Feature",
      "properties": {
        "NAME": "临湘市",
        "QUHUADAIMA": "430682",
        "Location": 6,
        "Alignment": 1
      },
      "geometry": {
        "type": "Point",
        "coordinates": [
          113.59011657158062,
          29.605213651824659
        ]
      }
    },
    {
      "type": "Feature",
      "properties": {
        "NAME": "汨罗市",
        "QUHUADAIMA": "430681",
        "Location": 2,
        "Alignment": 2
      },
      "geometry": {
        "type": "Point",
        "coordinates": [
          113.20413521709955,
          28.93628661571595
        ]
      }
        ...
```

### 代码

代码可见 [下载民政部行政区划数据](下载民政部行政区划数据.go)







