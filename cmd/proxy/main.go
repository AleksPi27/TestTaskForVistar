package main

import (
	"flag"
	"log"
	"math/rand"
	"net/http"
	//"net/url"
	//"os"
	//"os/signal"
	"time"

	"github.com/gorilla/websocket"
	proxy "test.task/backend/proxy"
)

type ordererStructure struct {
	instrument                        string
	numberOfOpenedOrdersPerInstrument int
}

var (
	addr                 = flag.String("addr", "localhost:8090", "http proxy service address")
	instrument           = flag.String("inst", "EURUSD", "instrument")
	seededRand           = rand.New(rand.NewSource(time.Now().UnixNano()))
	clientID             = seededRand.Uint32()
	upgrader             = websocket.Upgrader{}
	N                    = 5
	count                = 0
	openedOrdersByClient map[uint32]ordererStructure
)

func main() {
	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/proxy", connect)
	log.Printf("Waiting for connections on %s/proxy", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))

	//interrupt := make(chan os.Signal, 1)
	//signal.Notify(interrupt, os.Interrupt)
	//
	//u := url.URL{Scheme: "ws", Host: *addr, Path: "/proxy"}
	//log.Printf("connecting to %s", u.String())
}

func connect(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			break
		}

		if count < N {
			req := proxy.DecodeOrderRequest(message)
			log.Printf("recv: %v", req)
			count++
			log.Printf("Count of clients connected: %v", count)

			_ = req
			//var code int = 0
			//if
			res := proxy.OrderResponse {
			ID:
				req.ID,
					Code: 0,
			}
			err = c.WriteMessage(mt, proxy.EncodeOrderResponse(res))
			if err != nil {
				log.Println("write:", err)
				continue
			}

			log.Printf("sent: %v", res)
		}

	}
}
//
//func checkOpenedOrders(req proxy.OrderRequest) {
//	if openedOrdersByClient[req.ClientID].numberOfOpenedOrdersPerInstrument < N {
//		openedOrdersByClient
//	openedOrdersByClient=append(openedOrdersByClient, make([req.ClientID]{req.Instrument,openedOrdersByClient[req.ClientID].numberOfOpenedOrdersPerInstrument++}))
//	}
//}
