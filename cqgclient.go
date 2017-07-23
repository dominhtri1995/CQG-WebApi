package main

import (
	"github.com/golang/protobuf/proto"
	"fmt"
	"time"
	"hash/fnv"
	"reflect"
	"github.com/golang/protobuf/descriptor"
	"github.com/gorilla/websocket"
)

var userLogonList []User
var metadataMap = make(map[uint32]*ContractMetadata)
var newOrderList []NewOrderCancelUpdateStatus
var cancelOrderList []NewOrderCancelUpdateStatus
var updateOrderList []NewOrderCancelUpdateStatus

var chanLogon = make(chan *ServerMsg)
var chanInformationReport = make(chan *ServerMsg)
var chanOrderSubscription = make(chan *ServerMsg)

func CQG_NewOrderRequest(id uint32, accountID int32, contractID uint32, clorderID string, orderType uint32, price int32, duration uint32, side uint32, qty uint32, is_manual bool, utc int64) (ordStatus NewOrderCancelUpdateStatus) {
	var c = make(chan NewOrderCancelUpdateStatus)
	NewOrderRequest(id, accountID, contractID, clorderID, orderType, price, duration, side, qty, is_manual, utc, c)
	select {
	case ordStatus = <-c:
		return ordStatus
	case <-getTimeOutChan():
		ordStatus.status = "rejected"
		ordStatus.reason = "time out"
	}
	return ordStatus
}
func CQG_GetPosition(account string, accountID int32) *User {
	for _, user := range userLogonList {
		if user.username == account && user.accountID == accountID {
			return &user
		}
	}
	return nil
}
func CQG_GetWorkingOrder(account string, accountID int32) *User {
	for _, user := range userLogonList {
		if user.username == account && user.accountID == accountID {
			return &user
		}
	}
	return nil
}
func CQG_CancelOrderRequest(id uint32, orderID string, accountID int32, oldClorID string, clorID string, utc int64) (ordStatus NewOrderCancelUpdateStatus) {
	var c = make(chan NewOrderCancelUpdateStatus)
	CancelOrderRequest(id, orderID, accountID, oldClorID, clorID, utc, c)
	select {
	case ordStatus = <-c:
		return ordStatus
	case <-getTimeOutChan():
		ordStatus.status = "rejected"
		ordStatus.reason = "time out"
	}
	return ordStatus
}
func CQG_UpdateOrderRequest(id uint32, orderID string, accountID int32, oldClorID string, clorID string, utc int64,
	qty uint32, limitPrice int32, stopPrice int32, duration uint32, ) (ordStatus NewOrderCancelUpdateStatus) {

	var c = make(chan NewOrderCancelUpdateStatus)
	UpdateOrderRequest(id, orderID, accountID, oldClorID, clorID, utc, qty, limitPrice, stopPrice, duration, c)
	select {
	case ordStatus = <-c:
		return ordStatus
	case <-getTimeOutChan():
		ordStatus.status = "rejected"
		ordStatus.reason = "time out"
	}
	return ordStatus
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
func RecvMessage() {
	for {

		msg := RecvMessageOne()
		if (msg == nil) {
			return
		}
		v := reflect.ValueOf(*msg)
		//t := reflect.TypeOf(*msg)
		_, md := descriptor.ForMessage(msg)
		if msg.OrderStatus != nil || msg.PositionStatus != nil{
			//Save contractMetadata from order_status
			for _, orderStatus := range msg.GetOrderStatus() {
				for _, metadata := range orderStatus.GetContractMetadata() {
					metadataMap[metadata.GetContractId()] = metadata
				}
			}
			//Save contractMetadata from position_status
			for _, position := range msg.GetPositionStatus() {
				metadata := position.GetContractMetadata()
				metadataMap[metadata.GetContractId()] = metadata
			}
		}
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).Kind() != reflect.Struct && !v.Field(i).IsNil() {
				//fmt.Println(md.GetField()[i].GetName())
				switch md.GetField()[i].GetName() {
				case "logon_result":
					chanLogon <- msg
				case "logged_off":

				case "information_report":
					if (msg.GetInformationReport()[0].GetStatusCode() == 0 ) {
						metadata := msg.GetInformationReport()[0].GetSymbolResolutionReport().GetContractMetadata()
						metadataMap[metadata.GetContractId()] = metadata
						chanInformationReport <- msg
					} else {
						fmt.Println("Error Information Request")
					}
				case "position_status":
					fmt.Println("in position snapshot")
					for _, position := range msg.GetPositionStatus() {
						//update position
						if position.GetIsSnapshot() == true {
							accountID := position.GetAccountId()
							if len(position.GetOpenPosition()) > 0 {
								var userPosition Position
								userPosition.contractID = position.GetContractId()

								for id, metadata := range metadataMap {
									if id == userPosition.contractID {
										userPosition.symbol = metadata.GetTitle()
										userPosition.productDescription = metadata.GetDescription()
										userPosition.priceScale = metadata.GetCorrectPriceScale()
										userPosition.tickValue = metadata.GetTickValue()
									}
								}

								short := position.GetIsShortOpenPosition()
								if short == true {
									userPosition.side = "SELL"
								} else {
									userPosition.side = "BUY"
								}

								for _, openPosition := range position.GetOpenPosition() {
									userPosition.quantity += openPosition.GetQty()
									userPosition.price += openPosition.GetPrice() * float64(openPosition.GetQty())
								}
								//averaging out the price
								userPosition.price /= float64(userPosition.quantity)

								//add to Position list of user
								for k := range userLogonList {
									user := &userLogonList[k]
									if userLogonList[k].accountID == accountID {
										fmt.Println("add 1 position ", userPosition.symbol, userPosition.contractID)
										user.positionList = append(user.positionList, userPosition)
									}
								}

							} else {
								continue
							}
						} else {

						}
					}

				case "order_status":

					for _, orderStatus := range msg.GetOrderStatus() {
						if (orderStatus.GetIsSnapshot() == false) { //Fill Update
							clorIDTransaction := orderStatus.GetTransactionStatus()[0].GetClOrderId()
							switch TransactionStatus_Status_name[int32(orderStatus.GetTransactionStatus()[0].GetStatus())] {
							case TransactionStatus_REJECTED.String():
								for j := range newOrderList {
									noq := newOrderList[j]
									if noq.clorderID == clorIDTransaction {
										noq.status = "rejected"
										noq.reason = orderStatus.GetTransactionStatus()[0].GetTextMessage()
										noq.channel <- noq
										newOrderList = append(newOrderList[:j], newOrderList[j+1:]...)
										break
									}
								}
							case TransactionStatus_ACK_PLACE.String():
								clorIDOrder := orderStatus.GetOrder().GetClOrderId()
								for j := range newOrderList {
									noq := &newOrderList[j]
									if noq.clorderID == clorIDOrder {
										noq.status = "ok"
										noq.channel <- *noq
										newOrderList = append(newOrderList[:j], newOrderList[j+1:]...)
									}
								}

								accountID := orderStatus.GetOrder().GetAccountId()

								var wo WorkingOrder
								wo.orderID = orderStatus.GetOrderId()
								wo.chainOrderID = orderStatus.GetChainOrderId()
								wo.clorID = orderStatus.GetOrder().GetClOrderId()
								wo.contractID = orderStatus.GetOrder().GetContractId()
								wo.quantity = orderStatus.GetRemainingQty()
								wo.ordType = orderStatus.GetOrder().GetOrderType()
								wo.timeInForce = orderStatus.GetOrder().GetDuration()

								var price int32
								if (wo.ordType == 2) {
									price = orderStatus.GetOrder().GetLimitPrice()
								} else if (wo.ordType == 3 || wo.ordType == 4 ) {
									price = orderStatus.GetOrder().GetStopPrice()
								}
								wo.sideNum = orderStatus.GetOrder().GetSide()
								wo.side = Order_Side_name[int32(wo.sideNum)]

								for _, metadata := range metadataMap {
									if (metadata.GetContractId() == wo.contractID) {
										wo.symbol = metadata.GetTitle()
										wo.productDescription = metadata.GetDescription()
										wo.price = float64(price) * metadata.GetCorrectPriceScale()
									}
								}
								for k := range userLogonList {
									user := userLogonList[k]
									if user.accountID == accountID {
										user.workingOrderList = append(user.workingOrderList, wo)
									}
								}

							case TransactionStatus_FILL.String():
								accountID := orderStatus.GetOrder().GetAccountId()
								chainOrderID := orderStatus.GetChainOrderId()
								for _, user := range userLogonList {
									if user.accountID == accountID {
										for i := range user.workingOrderList {
											wo := &user.workingOrderList[i]
											if wo.chainOrderID == chainOrderID {
												wo.quantity = orderStatus.GetRemainingQty()

												if wo.quantity == 0 {
													user.workingOrderList = append(user.workingOrderList[:i], user.workingOrderList[i+1:]...)
												}
											}
										}
									}
								}

								//fillQuantity := orderStatus.GetFillQty()
								//fillPrice := orderStatus.GetAvgFillPrice()
								// Notification here

							case TransactionStatus_ACK_CANCEL.String():
								for j := range cancelOrderList {
									coq := &cancelOrderList[j]
									if coq.clorderID == clorIDTransaction {
										coq.status = "ok"
										coq.channel <- *coq
										cancelOrderList = append(cancelOrderList[:j], cancelOrderList[j+1:]...)

									}
								}
							case TransactionStatus_REJECT_CANCEL.String():
								for j := range cancelOrderList {
									coq := &cancelOrderList[j]
									if coq.clorderID == clorIDTransaction {
										coq.status = "rejected"
										coq.reason = orderStatus.GetTransactionStatus()[0].GetTextMessage()
										coq.channel <- *coq
										cancelOrderList = append(cancelOrderList[:j], cancelOrderList[j+1:]...)

									}
								}
							case TransactionStatus_ACK_MODIFY.String():
								clorIDOrder := orderStatus.GetOrder().GetClOrderId()
								for j := range updateOrderList {
									moq := &updateOrderList[j]
									if moq.clorderID == clorIDOrder {
										moq.status = "ok"
										moq.channel <- *moq
										updateOrderList = append(updateOrderList[:j], updateOrderList[j+1:]...)
									}
								}
								//account := orderStatus.GetEnteredByUser()
								accountID := orderStatus.GetOrder().GetAccountId()
								chainOrderID := orderStatus.GetChainOrderId()
								for _, user := range userLogonList {
									if user.accountID == accountID {
										for k := range user.workingOrderList {
											wo := &user.workingOrderList[k]
											if wo.chainOrderID == chainOrderID {
												wo.orderID = orderStatus.GetOrderId()
												wo.clorID = orderStatus.GetOrder().GetClOrderId()
												wo.quantity = orderStatus.GetRemainingQty()
												wo.timeInForce = orderStatus.GetOrder().GetDuration()
												ordType := orderStatus.GetOrder().GetOrderType()

												if (ordType == 2) {
													wo.price = float64(orderStatus.GetOrder().GetLimitPrice()) * wo.priceScale
												} else if (wo.ordType == 3 || wo.ordType == 4 ) {
													wo.price = float64(orderStatus.GetOrder().GetStopPrice()) * wo.priceScale
												}
											}
										}
									}
								}
							case TransactionStatus_REJECT_MODIFY.String():
								for j := range updateOrderList {
									moq := &updateOrderList[j]
									if moq.clorderID == clorIDTransaction {
										moq.status = "rejected"
										moq.reason = orderStatus.GetTransactionStatus()[0].GetTextMessage()
										moq.channel <- *moq
										updateOrderList = append(updateOrderList[:j], updateOrderList[j+1:]...)
									}
								}
							}
						} else { //Trade subscription snapshot
							if orderStatus.GetStatus() == 3 {
								fmt.Println("in order status snapshot")
								//account := orderStatus.GetEnteredByUser()
								accountID := orderStatus.GetOrder().GetAccountId()

								var wo WorkingOrder
								wo.orderID = orderStatus.GetOrderId()
								wo.chainOrderID = orderStatus.GetChainOrderId()
								wo.clorID = orderStatus.GetOrder().GetClOrderId()
								wo.contractID = orderStatus.GetOrder().GetContractId()
								wo.quantity = orderStatus.GetRemainingQty()
								wo.ordType = orderStatus.GetOrder().GetOrderType()
								wo.timeInForce = orderStatus.GetOrder().GetDuration()

								var price int32
								if (wo.ordType == 2) {
									price = orderStatus.GetOrder().GetLimitPrice()
								} else if (wo.ordType == 3 || wo.ordType == 4 ) {
									price = orderStatus.GetOrder().GetStopPrice()
								}
								wo.sideNum = orderStatus.GetOrder().GetSide()
								wo.side = Order_Side_name[int32(wo.sideNum)]

								for _, metadata := range metadataMap {
									if metadata.GetContractId() == wo.contractID {
										wo.symbol = metadata.GetTitle()
										wo.productDescription = metadata.GetDescription()
										wo.priceScale = metadata.GetCorrectPriceScale()
										wo.price = float64(price) * wo.priceScale
									}
								}

								for k := range userLogonList {
									if userLogonList[k].accountID == accountID {
										userLogonList[k].workingOrderList = append(userLogonList[k].workingOrderList, wo)
									}
								}
							}
						}
					}
				case "trade_subscription_status":
				case "trade_snapshot_completion":
					chanOrderSubscription <- msg
				case "user_message":
					msgType := msg.GetUserMessage()[0].GetMessageType()
					if msgType == 1 {
						//*********** CRITICAL ERROR ALERT ADMIN IMMEDIATELY
						fmt.Println("*********** CRITICAL ERROR ALERT ADMIN IMMEDIATELY *******")
						fmt.Println("Try logoff and logon again")
					}
				}
			}
		}
	}
}
func RecvMessageOne() (msg *ServerMsg) {
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

func getTimeOutChan() chan bool {
	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(10 * time.Second)
		timeout <- true
	}()
	return timeout
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
	username         string
	password         string
	accountID        int32
	workingOrderList []WorkingOrder
	positionList     []Position
}

type WorkingOrder struct {
	orderID      string // Used to cancel order or request order status later
	clorID       string
	chainOrderID string
	ordStatus    string
	quantity     uint32
	side         string
	sideNum      uint32
	ordType      uint32
	timeInForce  uint32
	price        float64
	contractID   uint32

	symbol             string
	productDescription string
	priceScale         float64
}
type Position struct {
	quantity           uint32
	side               string
	price              float64
	contractID         uint32
	symbol             string
	productDescription string
	priceScale         float64
	tickValue          float64
}
type NewOrderCancelUpdateStatus struct {
	clorderID string
	status    string
	reason    string
	channel   chan NewOrderCancelUpdateStatus
}
