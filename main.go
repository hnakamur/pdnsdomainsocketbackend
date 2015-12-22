package main

import (
	"flag"
	"encoding/json"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
)

// NOTE: とりあえずデコードだけできればよいので
// http://mattyjwilliams.blogspot.jp/2013/01/using-go-to-unmarshal-json-lists-with.html
// のSolutionB: Mixed Type structの方式で、各methodのパラメータに対応する
// フィールドをごちゃ混ぜで持つようにします。
type RequestParams struct {
	Path string `json:"path"`
	QType string `json:"qtype"`
	QName string `json:"qname"`
	Remote string `json:"remote"`
	Local string `json:"local"`
	RealRemote string `json:"real-remote"`
	ZoneID int `json:"zone-id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type Request struct {
	Method string `json:"method"`
	Parameters RequestParams `json:"parameters"`
}

type ResponseRecord struct {
	QType string `json:"qtype"`
	QName string `json:"qname"`
	Content string `json:"content"`
	TTL int `json:"ttl"`
}

type Response struct {
	Results []ResponseRecord `json:"result"`
}

var socketFilePath string

// シリアル番号の下2桁
var serialSeq int
var serialLock sync.Mutex

func getSerial() int {
	serialLock.Lock()
	serialSeq++
	serialLock.Unlock()
	return serialSeq
}

var hostnameIPMaps map[string]string

func init() {
	hostnameIPMaps = map[string]string{
		"foo.example.com": "192.50.100.1",
		"bar.example.com": "192.50.100.2",
		"baz.example.com": "192.50.100.3",
	}
}

func echoServer(c net.Conn) {
	for {
		buf := make([]byte, 512)
		nr, err := c.Read(buf)
		if err != nil {
			return
		}

		data := buf[0:nr]
		var req Request
		err = json.Unmarshal(data, &req)
		if err != nil {
			log.Println("failed to unmarshal json: ", err)
		}

		log.Printf("Server got: %s", string(data))
		switch req.Method {
		case "initialize":
			resp := `{"result":true}`
			_, err = c.Write([]byte(resp))
			if err != nil {
				log.Println("Failed to write initialize response: ", err)
			}
		case "lookup":
			resp := Response{
				Results: []ResponseRecord{
					{
						QType: req.Parameters.QType,
						QName: req.Parameters.QName,
						Content: hostnameIPMaps[req.Parameters.QName],
						TTL: 300,
					},
				},
			}
			encoder := json.NewEncoder(c)
			err := encoder.Encode(resp)
			if err != nil {
				log.Println("Failed to write lookup response: ", err)
			}
		case "getDomainMetadata":
			switch req.Parameters.Kind {
			case "PRESIGNED":
				resp := `{"result":[]}`
				_, err = c.Write([]byte(resp))
				if err != nil {
					log.Println("Failed to write getDomainMetadata PRESIGNED response: ", err)
				}
			case "SOA-EDIT":
				resp := `{"result":[]}`
				_, err = c.Write([]byte(resp))
				if err != nil {
					log.Println("Failed to write getDomainMetadata SOA-EDIT response: ", err)
				}
			default:
				if err != nil {
					log.Println("unsupported kind for getDomainMetadata: ", req.Parameters.Kind)
				}
			}
		case "calculateSOASerial":
			serial := getSerial()
			resp := `{"result":` + strconv.Itoa(serial) + `}`
			_, err = c.Write([]byte(resp))
			if err != nil {
				log.Println("Failed to write calculateSOASerial response: ", err)
			}
		default:
			log.Println("unsupported method: ", req.Method)
		}
	}
}

func main() {
	flag.StringVar(&socketFilePath, "socketfile", "/var/run/pdns/pdnsbackend.sock", "file path of domain socket")
	flag.Parse()

	f, err := os.OpenFile("/var/log/pdns/pdnsdomainsocketbackend.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("error opening file :", err.Error())
	}
	defer f.Close()
	log.SetOutput(f)

	log.Println("pdnsdomainsocketbackend server start")

	l, err := net.Listen("unix", socketFilePath)
	if err != nil {
		log.Fatal("listen error:", err)
	}

	for {
		fd, err := l.Accept()
		if err != nil {
			log.Fatal("accept error:", err)
		}

		go echoServer(fd)
	}
}
