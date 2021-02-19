package main

import (
	"flag"
	"log"
	"math/rand"
	"net"
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
	idClient                          uint32
	idOfOrder                         uint32
	instrument                        string
	numberOfOpenedOrdersPerInstrument int
	volume                            float64
	addr                              net.Addr
	//addr                              string
}

type clientIdAndConnectionPair struct {
	idOfOrder  uint32
	connection websocket.Conn
	mt         int
}

var (
	addr                 = flag.String("addr", "localhost:8090", "http proxy service address")
	addr1                = flag.String("addr1", "localhost:8080", "http service address")
	instrument           = flag.String("inst", "EURUSD", "instrument")
	seededRand           = rand.New(rand.NewSource(time.Now().UnixNano()))
	clientID             = seededRand.Uint32()
	upgrader             = websocket.Upgrader{}
	N                    = 10
	S                    = 1000.0
	count                = 0
	openedOrdersByClient []*ordererStructure
	connections          []*clientIdAndConnectionPair
	interval             = flag.Duration("inter", 2*time.Second, "interval of sending request")
)

func main() {
	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/proxy", connect)
	log.Printf("Waiting for connections on %s/proxy", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
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
			var code = checkOpenedOrders(req, *c)
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
			} else {
				var newConnection clientIdAndConnectionPair
				newConnection.idOfOrder = req.ID
				newConnection.connection = *c
				newConnection.mt = mt
				connections = append(connections, &newConnection)
				log.Println("Size of passed requests: ", len(openedOrdersByClient))
				log.Println("Successful open request")
				sendPassedRequestToServer(req)
			}
		}

	}
}

func checkOpenedOrders(req proxy.OrderRequest, connection websocket.Conn) uint16 {
	var isClientConnected = false
	for _, openedOrderByClient := range openedOrdersByClient {
		if (openedOrderByClient.idClient == req.ClientID) && (openedOrderByClient.instrument == req.Instrument) {
			isClientConnected = true
			if (openedOrderByClient.numberOfOpenedOrdersPerInstrument < N) && (openedOrderByClient.volume+req.Volume <= S) {
				log.Printf("Number of orders before increasing: %v", openedOrderByClient.numberOfOpenedOrdersPerInstrument)
				openedOrderByClient.numberOfOpenedOrdersPerInstrument++
				log.Printf("Number of orders after increasing: %v", openedOrderByClient.numberOfOpenedOrdersPerInstrument)
				log.Printf("Volume of orders before increasing: %v", openedOrderByClient.volume)
				openedOrderByClient.volume += req.Volume
				log.Printf("Volume of orders after increasing: %v", openedOrderByClient.volume)
				log.Print("openedOrderByClient: ", openedOrderByClient)
				return 0
			} else {
				if openedOrderByClient.numberOfOpenedOrdersPerInstrument >= N {
					log.Printf("openedOrderByClient.numberOfOpenedOrdersPerInstrument is more than N, exactly is: %v", openedOrderByClient.numberOfOpenedOrdersPerInstrument)
					return 1
				}
				if openedOrderByClient.volume+req.Volume > S {
					log.Printf("openedOrderByClient.volume is more than S, exactly is: %v", openedOrderByClient.volume+req.Volume)
					return 2
				}

			}
		}

	}
	if !isClientConnected {
		if req.Volume <= S {
			log.Printf("Old length is %v", len(openedOrdersByClient))
			var newClient ordererStructure
			newClient.idClient = req.ClientID
			newClient.idOfOrder = req.ID
			newClient.numberOfOpenedOrdersPerInstrument = 1
			newClient.instrument = req.Instrument
			newClient.volume = req.Volume
			newClient.addr = connection.RemoteAddr()
			log.Printf("Size of orders before appending: %v", len(openedOrdersByClient))
			openedOrdersByClient = append(openedOrdersByClient, &newClient)
			log.Printf("Size of orders after appending: %v", len(openedOrdersByClient))
			log.Printf("New length is %v", len(openedOrdersByClient))
			return 0
		} else {
			return 2
		}
	}
	log.Print("Nothing happened")
	return 3
}

func sendPassedRequestToServer(req proxy.OrderRequest) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	u := url.URL{Scheme: "ws", Host: *addr1, Path: "/sendPassedRequestToServer"}
	log.Printf("connecting to %s", u.String())
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	log.Printf("address: %v", c.RemoteAddr())
	ticker := time.NewTicker(*interval)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, mess, err := c.ReadMessage()
			if err != nil {
				log.Printf("recv error: %+v", err)
				return
			}
			log.Printf("recv from server: %v", proxy.DecodeOrderResponse(mess))
			answerFromServer := proxy.DecodeOrderResponse(mess)
			log.Printf("Size of connections: %v", len(connections))
			for _, connectionWithClient := range connections {
				//log.Printf("idOfOrder of cl")
				if connectionWithClient.idOfOrder == answerFromServer.ID {
					err := connectionWithClient.connection.WriteMessage(connectionWithClient.mt, proxy.EncodeOrderResponse(answerFromServer))
					if err != nil {
						log.Println("write to client:", err)
						continue
					}
					log.Printf("sent to client: %v", answerFromServer)
					for _, openedOrderByClient := range openedOrdersByClient {
						if openedOrderByClient.idOfOrder == answerFromServer.ID {
							if len(openedOrdersByClient) > 1 {
								openedOrderByClient = openedOrdersByClient[len(openedOrdersByClient)-1]
								openedOrdersByClient[len(openedOrdersByClient)-1] = nil
								openedOrdersByClient = openedOrdersByClient[:len(openedOrdersByClient)-1]
							} else{
								openedOrdersByClient=openedOrdersByClient[:0]
							}
						}
					}
					log.Printf("Size of opened connections after deleting one: %v",len(openedOrdersByClient))
					for _, connection := range connections {
						if connection.idOfOrder == answerFromServer.ID {
							if(len(connections)>1) {
								connection = connections[len(connections)-1]
								connections[len(connections)-1] = nil
								connections = connections[:len(connections)-1]
							} else{
								connections=connections[:0]
							}
						}
					}
					log.Printf("Size of connections after deleting connection: %v",len(connections))
				}
			}
		}
	}()

	select {
	case <-ticker.C:
		reqToServer := proxy.OrderRequest{
			ClientID:   req.ClientID,
			ID:         req.ID,
			ReqType:    req.ReqType,
			OrderKind:  req.OrderKind,
			Volume:     req.Volume,
			Instrument: req.Instrument,
		}
		err := c.WriteMessage(websocket.TextMessage, proxy.EncodeOrderRequest(reqToServer))
		if err != nil {
			log.Printf("send err: %v", err)
		} else {
			log.Printf("sent: %v", reqToServer)
		}
	case <-interrupt:
		log.Println("interrupt")
		err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Println("write close:", err)
			return
		}
		return
	}

}
