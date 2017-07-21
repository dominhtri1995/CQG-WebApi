package main

import (
	"flag"
	"net/url"
	"os"
	"os/signal"

	"github.com/gorilla/websocket"
	"github.com/golang/protobuf/proto"
	"fmt"
	"log"
	"bufio"
	"time"
	"hash/fnv"
	"reflect"
	"github.com/golang/protobuf/descriptor"
	"github.com/rs/xid"
)

var addr = flag.String("addr", "demoapi.cqg.com:443", "http service address")
var conn *websocket.Conn
var err error

var metaDataList []*ContractMetadata
var newOrderList []NewOrderCancelUpdateStatus
var cancelOrderList []NewOrderCancelUpdateStatus
var updateOrderList []NewOrderCancelUpdateStatus

var chanLogon = make(chan *ServerMsg)
var chanInformationReport= make(chan *ServerMsg)
var chanOrderSubscription = make (chan *ServerMsg)

func main() {
	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	u := url.URL{Scheme: "wss", Host: *addr, Path: ""}
	log.Printf("connecting to %s", u.String())

	conn, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer conn.Close()
	go RecvMessage()

	SendLogonMessage("VTechapi", "pass", "WebApiTest", "java-client")

	CQG_InformationRequest("BZU7", 1)
	CQG_NewOrderRequest(1,16958204,1, xid.New().String(),2, 4700,2,1,1,false, makeTimestamp() )
	//CQG_CancelOrderRequest(1,"799143186",16958204,"370da065-d10d-4041-8078-0bf629b87b5a",xid.New().String(),makeTimestamp())

	//CQG_PositionSubscription(hash(xid.New().String()),true)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
}

func CQG_OrderSubscription(id uint32, subscribe bool){
	var arr []uint32
	arr = append(arr, 1)
	clientMsg := &ClientMsg{
		TradeSubscription: []*TradeSubscription{
			{
				Id: &id,
				Subscribe: &subscribe,
				SubscriptionScope: arr,
			},
		},
	}
	SendMessage(clientMsg)
	_ = <- chanOrderSubscription
}
func CQG_PositionSubscription(id uint32, subscribe bool){
	var arr []uint32
	arr = append(arr, 2)
	clientMsg := &ClientMsg{
		TradeSubscription: []*TradeSubscription{
			{
				Id: &id,
				Subscribe: &subscribe,
				SubscriptionScope: arr,
			},
		},
	}
	SendMessage(clientMsg)
}
func CQG_NewOrderRequest(id uint32, accountID int32, contractID uint32, clorderID string, orderType uint32,price int32, duration uint32, side uint32, qty uint32, is_manual bool,utc int64) {
	clientMsg := &ClientMsg{
		OrderRequest: []*OrderRequest{
			{
				RequestId: &id,
				NewOrder: &NewOrder{
					Order: &Order{
						AccountId: &accountID,
						WhenUtcTime: &utc,
						ContractId: &contractID,
						ClOrderId: &clorderID,
						OrderType: &orderType,
						Duration: &duration,
						Side: &side,
						Qty: &qty,
						IsManual: &is_manual,
					},
				},
			},
		},
	}

	if(orderType == 2){
		clientMsg.GetOrderRequest()[0].GetNewOrder().GetOrder().LimitPrice = &price
	} else if( orderType == 3 || orderType == 4){
		clientMsg.GetOrderRequest()[0].GetNewOrder().GetOrder().StopPrice = &price
	}

	SendMessage(clientMsg)
}
func CQG_CancelOrderRequest(id uint32,orderID string, accountID int32, oldClorID string, clorID string, utc int64){
	clientMsg := &ClientMsg{
		OrderRequest: []*OrderRequest{
			{
				RequestId: &id,
				CancelOrder: &CancelOrder{
					OrderId: &orderID,
					AccountId: &accountID,
					OrigClOrderId: &oldClorID,
					ClOrderId: &clorID,
					WhenUtcTime: &utc,
				},
			},
		},
	}
	SendMessage(clientMsg)
}
func CQG_UpdateOrderRequest(id uint32,orderID string, accountID int32, oldClorID string,clorID string,utc int64,
							qty uint32,limitPrice int32, stop_price int32, duration uint32,){
	/* Pass in qty,limitprice, stopprice and duration if you want to change these elements.
	// Otherwise pass in 0
	 */
	clientMsg := &ClientMsg{
		OrderRequest: []*OrderRequest{
			{
				RequestId: &id,
				ModifyOrder: &ModifyOrder{
					OrderId: &orderID,
					AccountId: &accountID,
					OrigClOrderId: &oldClorID,
					ClOrderId: &clorID,
					WhenUtcTime: &utc,
				},
			},
		},
	}
	if(qty != 0){
		clientMsg.GetOrderRequest()[0].GetModifyOrder().Qty = &qty
	}
	if(limitPrice != 0){
		clientMsg.GetOrderRequest()[0].GetModifyOrder().LimitPrice = &limitPrice
	}
	if(stop_price != 0){
		clientMsg.GetOrderRequest()[0].GetModifyOrder().StopPrice = &stop_price
	}
	if(duration != 0){
		clientMsg.GetOrderRequest()[0].GetModifyOrder().Qty = &duration
	}
	SendMessage(clientMsg)
}
func CQG_InformationRequest(symbol string, id uint32)  {
	clientMsg := &ClientMsg{
		InformationRequest: []*InformationRequest{
			{Id: &id,
				SymbolResolutionRequest: &SymbolResolutionRequest{
					Symbol: &symbol,
				},
			},
		},
	}
	SendMessage(clientMsg)
	_ = <- chanInformationReport
}
func SendLogonMessage(username string, password string, clientAppID string, clientVersion string) {
	LogonMessage := &ClientMsg{
		Logon: &Logon{UserName: &username,
			Password:           &password,
			ClientAppId:        &clientAppID,
			ClientVersion:      &clientVersion},
	}

	SendMessage(LogonMessage)
	msg:= <- chanLogon
	if msg.LogonResult.GetResultCode() == 0  {
		fmt.Printf("Logon Successfully!!! Let's make America great again \n")
		CQG_OrderSubscription(hash(xid.New().String()),true)
	} else {
		fmt.Printf("Logon failed !! It's Obama's fault \n")
	}

}
func SendMessage(message *ClientMsg) {
	out, _ := proto.Marshal(message)
	err := conn.WriteMessage(websocket.BinaryMessage, []byte(out))
	if err != nil {
		fmt.Println("error:", err)
		return
	} else {
		fmt.Printf("send: %s\n", *message)
	}
}
func RecvMessage(){
	for {

		msg := RecvMessageOne()
		if(msg == nil){
			return
		}
		v := reflect.ValueOf(*msg)
		//t := reflect.TypeOf(*msg)
		_,md := descriptor.ForMessage(msg)

		for i:=0; i< v.NumField() ; i++{
			if v.Field(i).Kind()!= reflect.Struct && !v.Field(i).IsNil(){
				//fmt.Println(md.GetField()[i].GetName())
				switch md.GetField()[i].GetName(){
				case "logon_result":
					chanLogon <- msg
				case "logged_off":

				case "information_report":
					if (msg.GetInformationReport()[0].GetStatusCode() == 0 ) {
						metadata := msg.GetInformationReport()[0].GetSymbolResolutionReport().GetContractMetadata()
						metaDataList = append(metaDataList, metadata)
						chanInformationReport <- msg
					}else {
						fmt.Println("Error Information Request")
					}
				case "position_status":
					fmt.Printf("Number of position: ")
					fmt.Println(len(msg.GetPositionStatus()))

				case "order_status":
					for _, orderStatus := range msg.GetOrderStatus(){

						if(orderStatus.GetIsSnapshot() == false){ //Fill Update
							clorID :=orderStatus.GetOrder().GetClOrderId()

							switch orderStatus.GetTransactionStatus()[0].GetStatus(){
							case TransactionStatus_REJECTED:
								for _, noq := range newOrderList{
									if(noq.clorID == clorID){
										noq.status ="rejected"
										noq.channel <- noq
									}
								}
							case TransactionStatus_ACK_PLACE:
								for _, noq := range newOrderList{
									if(noq.clorID == clorID){
										noq.status ="ok"
										noq.channel <- noq
									}
								}
							case TransactionStatus_ACK_CANCEL:
								for _, coq := range cancelOrderList{
									if(coq.clorID == clorID){
										coq.status ="ok"
										coq.channel <- coq
									}
								}
							case TransactionStatus_REJECT_CANCEL:
								for _, coq := range cancelOrderList{
									if(coq.clorID == clorID){
										coq.status ="rejected"
										coq.channel <- coq
									}
								}
							case TransactionStatus_ACK_MODIFY:
								for _, moq := range updateOrderList{
									if(moq.clorID == clorID){
										moq.status ="ok"
										moq.channel <- moq
									}
								}
							case TransactionStatus_REJECT_MODIFY:
								for _, moq := range updateOrderList{
									if(moq.clorID == clorID){
										moq.status ="rejected"
										moq.channel <- moq
									}
								}
							}
						} else { //Trade subscription snapshot

						}


						//Save contractMetadata
						for _, contractMetadata := range orderStatus.GetContractMetadata(){
							metaDataList = append(metaDataList, contractMetadata)
							fmt.Println("save metadata")
						}
					}
				case "trade_subscription_status":
				case "trade_snapshot_completion":
					chanOrderSubscription <- msg
				}
			}
		}
	}
}
func RecvMessageOne()(msg *ServerMsg){
	_, message, err := conn.ReadMessage()
	if err != nil {
		fmt.Println("read error:", err)
		return
	}
	msg = &ServerMsg{}
	proto.Unmarshal(message, msg)
	fmt.Printf("recv: %s \n", msg)
	return msg
}
func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

type User struct {
	account string
	accountID string
	workingOrderList []WorkingOrder
}

type WorkingOrder struct{
	orderID         string // Used to cancel order or request order status later
	clorID 			string
	price           string
	ordStatus       string
	quantity        string
	symbol          string
	productMaturity string
	exchange 		string
	productType     string
	side            string
	sideNum			string
	ordType 		string
	timeInForce 	string
	contractID		string
}

type NewOrderCancelUpdateStatus struct{
	clorID 			string
	status 			string
	channel         chan NewOrderCancelUpdateStatus
}

