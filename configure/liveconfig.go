package configure

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

/*
{
	[
	{
	"application":"live",
	"live":"on",
	"hls":"on",
	"static_push":["rtmp://xx/live"]
	}
	]
}
*/
type Application struct {
	Appname     string
	Liveon      string
	Hlson       string
	Static_push []string
}

type ServerCfg struct {
	Server []Application
}

var RtmpServercfg ServerCfg

func LoadConfig(configfilename string) error {
	fmt.Printf("starting load configure file(%s)...\n", configfilename)
	data, err := ioutil.ReadFile(configfilename)
	if err != nil {
		fmt.Printf("Read file %s error, %v\n", configfilename, err)
		return err
	}

	fmt.Printf("loadconfig:%s\n", string(data))
	err = json.Unmarshal(data, &RtmpServercfg)
	if err != nil {
		fmt.Printf("json.Unmarshal error, %v\n", err)
		return err
	}
	fmt.Printf("get config json data:%v\n", RtmpServercfg)
	return nil
}

func CheckAppName(appname string) bool {
	for _, app := range RtmpServercfg.Server {
		if (app.Appname == appname) && (app.Liveon == "on") {
			return true
		}
	}
	return false
}

func GetStaticPushUrlList(appname string) ([]string, bool) {
	for _, app := range RtmpServercfg.Server {
		if (app.Appname == appname) && (app.Liveon == "on") {
			if len(app.Static_push) > 0 {
				return app.Static_push, true
			} else {
				return nil, false
			}
		}

	}
	return nil, false
}
