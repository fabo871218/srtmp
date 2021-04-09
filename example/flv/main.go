package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/fabo871218/srtmp/httpflv"
)

func getflv() {
	response, err := http.Get("http://sf1-hscdn-tos.pstatp.com/obj/media-fe/xgplayer_doc_video/flv/xgplayer-demo-360p.flv")
	if err != nil {
		panic(err)
	}

	if response.StatusCode != 200 {
		panic(response.StatusCode)
	}

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	w, err := os.OpenFile("/Users/fabojiang/Desktop/xigua.flv", os.O_CREATE|os.O_RDWR, 0755)
	if err != nil {
		panic(err)
	}
	defer w.Close()

	w.Write(data)
}

func main() {
	//getflv()

	addr := fmt.Sprintf(":8090")
	httpflv.ListenAndServe(addr)
}
