package main

import (
	"flag"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"

	//"net/url"
	//"os"
	//"os/signal"
	"time"

	"github.com/gorilla/websocket"
	proxy "test.task/backend/proxy"
)

type ordererStructure struct {
	idOrderer                         uint32
	instrument                        string
	numberOfOpenedOrdersPerInstrument int
	volume                            float64
}

var (
	addr                 = flag.String("addr", "localhost:8090", "http proxy service address")
	instrument           = flag.String("inst", "EURUSD", "instrument")
	seededRand           = rand.New(rand.NewSource(time.Now().UnixNano()))
	clientID             = seededRand.Uint32()
	upgrader             = websocket.Upgrader{}
	N                    = 5
	S                    = 200.0
	count                = 0
	openedOrdersByClient []*ordererStructure
	interval              = flag.Duration("inter", 2*time.Second, "interval of sending request")
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

		req := proxy.DecodeOrderRequest(message)
		if req.ReqType == 1 {
			var code uint16 = checkOpenedOrders(req)
			log.Printf("recv: %v", req)
			count++
			log.Printf("Count of clients connected: %v", len(openedOrdersByClient))

			_ = req
			if code > 0 {
				log.Println("Unsuccessful open request")
				res := proxy.OrderResponse{
					ID:
					req.ID,
					Code: code,
				}
				err = c.WriteMessage(mt, proxy.EncodeOrderResponse(res))
				if err != nil {
					log.Println("write:", err)
					continue
				}

				log.Printf("sent: %v", res)
			} else{
				log.Println("Successful open request")
				connectToServer(req)
			}
		}

	}
}

func checkOpenedOrders(req proxy.OrderRequest) uint16 {
	var isClientConnected bool = false
	for _, openedOrderByClient := range openedOrdersByClient {
		if (openedOrderByClient.idOrderer == req.ClientID) && (openedOrderByClient.instrument == req.Instrument) {
			isClientConnected = true
			if (openedOrderByClient.numberOfOpenedOrdersPerInstrument < N) && (openedOrderByClient.volume+req.Volume <= S) {
				openedOrderByClient.numberOfOpenedOrdersPerInstrument++
				openedOrderByClient.volume += req.Volume
				log.Print("openedOrderByClient: ", openedOrderByClient)
				return 0
			} else {
				if openedOrderByClient.numberOfOpenedOrdersPerInstrument >= N {
					log.Printf("openedOrderByClient.numberOfOpenedOrdersPerInstrument: %v", openedOrderByClient.numberOfOpenedOrdersPerInstrument)
					return 1
				}
				if openedOrderByClient.volume+req.Volume > S {
					log.Printf("openedOrderByClient.volume: %v", openedOrderByClient.volume+req.Volume)
					return 2
				}

			}
		}

	}
	if !isClientConnected {
		if req.Volume <= S {
			log.Printf("Old length is %v", len(openedOrdersByClient))
			var newClient ordererStructure
			newClient.idOrderer = req.ClientID
			newClient.numberOfOpenedOrdersPerInstrument = 1
			newClient.instrument = req.Instrument
			newClient.volume = req.Volume
			openedOrdersByClient = append(openedOrdersByClient, &newClient)
			log.Printf("New length is %v", len(openedOrdersByClient))
			return 0
		} else {
			return 2
		}
	}
	log.Print("Nothing happened")
	return 3
}

func connectToServer(req proxy.OrderRequest) {
	var addr1                  = flag.String("addr1", "localhost:8080", "http service address")
	interrupt:=make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	u := url.URL{Scheme: "ws", Host: *addr1, Path: "/connectToServer"}
	log.Printf("connecting to %s", u.String())
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()
	done := make(chan struct{})
	//возможно
	go func() {
		defer close(done)
		for {
			_, mess, err := c.ReadMessage()
			if err != nil {
				log.Printf("recv error: %+v", err)
				return
			}
			log.Printf("recv from server: %v", proxy.DecodeOrderResponse(mess))
		}
	}()
	ticker:=time.NewTicker(*interval)
	//var id = uint32(time.Now().UTC().Unix())


		select {
		case <-done:
			return
		case <-ticker.C:
			reqToServer:=proxy.OrderRequest{
			ClientID: req.ClientID,
			ID: req.ID,
			ReqType: req.ReqType,
			OrderKind: req.OrderKind,
			Volume: req.Volume,
			Instrument: req.Instrument,
			}
			err:=c.WriteMessage(websocket.TextMessage, proxy.EncodeOrderRequest(reqToServer))
			if err != nil {
				log.Printf("send err: %v", err)
			} else {
				log.Printf("sent: %v", reqToServer)
			}
			case <-interrupt:
				log.Println("interrupt")
				err:=c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure,""))
				if err != nil{
					log.Println("write close:",err)
					return
				}
				select {
				case <- done:
					case <- time.After(time.Second):
				}
				return
		}


}
//
//func handleAndswerFromServerAndSendItToClient(w http.ResponseWriter, r *http.Request){
//	c, err :=upgrader.Upgrade(w,r,nil)
//	if err != nil{
//		log.Print("upgrade:",err)
//		return
//	}
//	defer c.Close()
//	for {
//		mt, message, err:=c.ReadMessage()
//		if err !=nil{
//			break
//		}
//		responseFromServer:=proxy.DecodeOrderRequest(message)
//		log.Printf("recv: %v",responseFromServer)
//		_ = responseFromServer
//
//		responseToClient:= proxy.OrderResponse{
//			ID: responseFromServer.ID,
//			Code: responseFromServer.
//		}
//	}
//}
