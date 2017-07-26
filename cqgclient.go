package main

import (
	"github.com/golang/protobuf/proto"
	"golang.org/x/sync/syncmap"
	"fmt"
	"time"
	"hash/fnv"
	"reflect"
	"github.com/golang/protobuf/descriptor"
	"github.com/gorilla/websocket"
	"github.com/rs/xid"
	"flag"
	"log"
	"net/url"
	"sync"
	"os"
)

var cqgAccountMap = NewCQGAccountMap()
var userMap = NewUserMap()

var newOrderMap syncmap.Map
var cancelOrderMap syncmap.Map
var updateOrderMap syncmap.Map
var informationRequestMap syncmap.Map

var addr = flag.String("addr", "demoapi.cqg.com:443", "http service address")

func CQG_StartWebApi(username string, password string, accountID int32) int {

	cqgAccount := NewCQGAccount()
	if _, ok := cqgAccountMap.accountMap[username]; !ok {
		err := startNewConnection(cqgAccount, username)

		if err == nil {
			return -1
		}

		cqgAccount.username = username
		cqgAccount.password = password
		cqgAccountMap.addAccount(cqgAccount)
		msg := CQG_SendLogonMessage(username, accountID, password, "WebApiTest", "java-client")
		if msg.LogonResult.GetResultCode() == 0 {
			fmt.Printf("%s Logon Successfully!!! Let's make America great again \n", username)
		} else {
			fmt.Printf("%s Logon failed !! It's Obama's fault \n", username)
			cqgAccountMap.removeAccount(username)
			//Close the socket
			err := cqgAccount.connWithLock.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("Send CloseMessage err :", err)
			}
			cqgAccount.connWithLock.conn.Close()
			return -1
		}

	} else {
		cqgAccount = cqgAccountMap.accountMap[username]
	}

	if _, ok := cqgAccount.userMap[accountID]; !ok {
		var user User
		user.username = username
		user.accountID = accountID
		cqgAccount.addUser(&user)
		userMap.addUser(&user)
		CQG_OrderSubscription(hash(xid.New().String()), true, username)
	}

	return 0
}
func startNewConnection(cqgAccount *CQGAccount, username string) *CQGAccount {

	flag.Parse()
	log.SetFlags(0)

	u := url.URL{Scheme: "wss", Host: *addr, Path: ""}
	log.Printf("connecting to %s", u.String())

	//establish connection and return a websocket to .conn of this account
	cqgAccount.connWithLock.conn, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Println("Error dial:", err)
		return nil
	}
	go RecvMessage(cqgAccount.connWithLock, username)

	return cqgAccount
}
func CQG_NewOrderRequest(id uint32, accountID int32, symbol string, clorderID string, orderType uint32, price int32, duration uint32, side uint32, qty uint32, is_manual bool, utc int64) (ordStatus NewOrderCancelUpdateStatus) {

	if checkUserLogonStatus(accountID) == -1 { // If user havent logon and no connection established
		ordStatus.status = "rejected"
		ordStatus.reason = "Connection not established. Please sign in"
		return
	}

	ifr := CQG_InformationRequest(symbol, 1, "VTechapi")

	if ifr.status == "ok" {
		var c = make(chan NewOrderCancelUpdateStatus)
		user, _ := userMap.getUser(accountID)
		NewOrderRequest(id, user.username, accountID, ifr.contractID, clorderID, orderType, price, duration, side, qty, is_manual, utc, c)
		select {
		case ordStatus = <-c:
			return ordStatus
		case <-getTimeOutChan():
			ordStatus.status = "rejected"
			ordStatus.reason = "time out"
		}
	} else {
		ordStatus.status = "rejected"
		ordStatus.reason = ifr.reason
	}

	return ordStatus
}
func CQG_GetPosition(accountID int32) *UserInfoRequest {
	if checkUserLogonStatus(accountID) == -1 { // If user havent logon and no connection established
		var uir UserInfoRequest
		uir.status = "rejected"
		uir.reason = "Connection not established. Please sign in"
		return &uir
	}

	if user, ok := userMap.getUser(accountID); ok {
		var uir UserInfoRequest
		uir.status = "ok"
		uir.username = user.username
		uir.accountID = user.accountID
		user.positionMap.Range(func(key, value interface{}) bool {
			position, _ := value.(*Position)
			uir.positionList = append(uir.positionList, *position)
			uir.collateralInfo = user.collateralInfo
			return true
		})
		return &uir
	}
	return nil
}
func CQG_GetWorkingOrder(accountID int32) *UserInfoRequest {
	if checkUserLogonStatus(accountID) == -1 { // If user havent logon and no connection established
		var uir UserInfoRequest
		uir.status = "rejected"
		uir.reason = "Connection not established. Please sign in"
		return &uir
	}

	if user, ok := userMap.getUser(accountID); ok {
		var uir UserInfoRequest
		uir.status = "ok"
		uir.username = user.username
		uir.accountID = user.accountID
		user.workingOrderMap.Range(func(key, value interface{}) bool {
			wo, _ := value.(*WorkingOrder)
			uir.workingOrderList = append(uir.workingOrderList, *wo)
			uir.collateralInfo = user.collateralInfo
			return true
		})
		return &uir
	}
	return nil
}
func CQG_GetCollateralInfo(accountID int32) *UserInfoRequest {
	if checkUserLogonStatus(accountID) == -1 { // If user havent logon and no connection established
		var uir UserInfoRequest
		uir.status = "rejected"
		uir.reason = "Connection not established. Please sign in"
		return &uir
	}

	if user, ok := userMap.getUser(accountID); ok {
		var uir UserInfoRequest
		uir.status = "ok"
		uir.collateralInfo = user.collateralInfo

		return &uir
	}
	return nil
}
func CQG_CancelOrderRequest(id uint32, orderID string, accountID int32, oldClorID string, clorID string, utc int64) (ordStatus NewOrderCancelUpdateStatus) {

	if checkUserLogonStatus(accountID) == -1 { // If user havent logon and no connection established
		ordStatus.status = "rejected"
		ordStatus.reason = "Connection not established. Please sign in"
		return
	}

	var c = make(chan NewOrderCancelUpdateStatus)
	user, _ := userMap.getUser(accountID)
	CancelOrderRequest(id, orderID, user.username, accountID, oldClorID, clorID, utc, c)
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

	if checkUserLogonStatus(accountID) == -1 { // If user havent logon and no connection established
		ordStatus.status = "rejected"
		ordStatus.reason = "Connection not established. Please sign in"
		return
	}

	user, _ := userMap.getUser(accountID) // get the user associated
	var c = make(chan NewOrderCancelUpdateStatus)
	UpdateOrderRequest(id, orderID, user.username, accountID, oldClorID, clorID, utc, qty, limitPrice, stopPrice, duration, c)
	select {
	case ordStatus = <-c:
		return ordStatus
	case <-getTimeOutChan():
		ordStatus.status = "rejected"
		ordStatus.reason = "time out"
	}
	return ordStatus
}

func SendMessage(message *ClientMsg, connWithLock ConnWithLock) {
	connWithLock.rwmux.Lock()
	out, _ := proto.Marshal(message)
	err := connWithLock.conn.WriteMessage(websocket.BinaryMessage, []byte(out))
	if err != nil {
		fmt.Println("CQG error:", err)
		log.Println("CQG error:", err)
		return
	} else {
		//fmt.Printf("send: %s\n", *message)
		log.Printf("send: %s\n", *message)
	}
	connWithLock.rwmux.Unlock()
}
func RecvMessage(connWithLock ConnWithLock, username string) {
	metadataMap := cqgAccountMap.accountMap[username].metadataMap
Loop:
	for {

		msg := RecvMessageOne(connWithLock)
		if _, ok := cqgAccountMap.accountMap[username]; !ok { //If the cqgAccount is not in monitor list anymore,
			// meaning failed logon- the connection is already closed
			break
		}
		if msg == nil { //connection error, try to reconnect
			recoverFromDisconnection(connWithLock, username)
			break
		}

		v := reflect.ValueOf(*msg)
		//t := reflect.TypeOf(*msg)
		_, md := descriptor.ForMessage(msg)
		if msg.OrderStatus != nil || msg.PositionStatus != nil {
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
					cqgAccountMap.accountMap[username].chanLogon <- msg
				case "logged_off":

				case "information_report":
					for _, inforeport := range msg.GetInformationReport() {
						id := inforeport.GetId()
						if ifr, ok := informationRequestMap.Load(id); ok {
							ifr, _ := ifr.(InformationRequestStatus)
							switch InformationReport_StatusCode_name[int32(inforeport.GetStatusCode())] {
							case InformationReport_FAILURE.String():
								fmt.Println("Error Information Request")
								ifr.status = "rejected"
								ifr.reason = inforeport.GetTextMessage()
								ifr.channel <- ifr
							case InformationReport_NOT_FOUND.String():
								ifr.status = "rejected"
								ifr.reason = inforeport.GetTextMessage()
								ifr.channel <- ifr
							case InformationReport_REQUEST_LIMIT_VIOLATION.String():
								ifr.status = "rejected"
								ifr.reason = "exceed request limit for the day"
								ifr.channel <- ifr
							default:
								metadata := inforeport.GetSymbolResolutionReport().GetContractMetadata()
								metadataMap[metadata.GetContractId()] = metadata
								ifr.status = "ok"
								ifr.contractID = metadata.GetContractId()
								ifr.channel <- ifr
							}
						} else {
							fmt.Println("unexpected information report message. error contact admin")
						}

					}
				case "position_status":
					for _, position := range msg.GetPositionStatus() {
						//update position
						if position.GetIsSnapshot() == true { // Snapshot
							accountID := position.GetAccountId()
							if len(position.GetOpenPosition()) > 0 {
								var userPosition Position
								userPosition.subPositionMap = make(map[int32]*OpenPosition)
								userPosition.contractID = position.GetContractId()

								if metadata, ok := metadataMap[userPosition.contractID]; ok {
									userPosition.symbol = metadata.GetTitle()
									userPosition.productDescription = metadata.GetDescription()
									userPosition.priceScale = metadata.GetCorrectPriceScale()
									userPosition.tickValue = metadata.GetTickValue()
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
									userPosition.subPositionMap[openPosition.GetId()] = openPosition
								}
								//averaging out the price
								userPosition.price /= float64(userPosition.quantity)

								//add to Position list of user
								if user, ok := userMap.getUser(accountID); ok {
									fmt.Println("add 1 position ", userPosition.symbol, userPosition.contractID)
									user.positionMap.Store(userPosition.contractID, &userPosition)
								}

							} else {
								continue
							}
						} else { //Position Update
							accountID := position.GetAccountId()
							if len(position.GetOpenPosition()) > 0 {
								var userPosition Position
								userPosition.subPositionMap = make(map[int32]*OpenPosition)
								userPosition.contractID = position.GetContractId()

								if metadata, ok := metadataMap[userPosition.contractID]; ok {
									userPosition.symbol = metadata.GetTitle()
									userPosition.productDescription = metadata.GetDescription()
									userPosition.priceScale = metadata.GetCorrectPriceScale()
									userPosition.tickValue = metadata.GetTickValue()
								}

								short := position.GetIsShortOpenPosition()
								if short == true {
									userPosition.side = "SELL"
									userPosition.shortBool = true
								} else {
									userPosition.side = "BUY"
									userPosition.shortBool = false
								}

								//add to Position list of user
								if user, ok := userMap.getUser(accountID); ok {
									if p, ok := user.positionMap.Load(userPosition.contractID); ok { //Contract already exist
										p, _ := p.(*Position)
										for _, openPosition := range position.GetOpenPosition() { //Update open_position/subposition
											if oldOpenPosition, ok := p.subPositionMap[openPosition.GetId()]; ok {
												oldOpenPosition.Qty = openPosition.Qty
												oldOpenPosition.Price = openPosition.Price

											} else { // add new open_position
												p.subPositionMap[openPosition.GetId()] = openPosition
											}
										}
										p.updatePriceAndQty()
										p.side = userPosition.side
										if p.quantity == 0 { //remove position if qty =0 square position
											user.positionMap.Delete(p.contractID)
										}

									} else { //New Position
										for _, openPosition := range position.GetOpenPosition() {
											userPosition.quantity += openPosition.GetQty()
											userPosition.price += openPosition.GetPrice() * float64(openPosition.GetQty())
											userPosition.subPositionMap[openPosition.GetId()] = openPosition
										}
										//averaging out the price
										userPosition.price /= float64(userPosition.quantity)
										user.positionMap.Store(userPosition.contractID, &userPosition)
									}

								}

							} else {
								continue
							}
						}
					}

				case "order_status":
					for _, orderStatus := range msg.GetOrderStatus() {
						if orderStatus.GetIsSnapshot() == false { //Fill Update
							clorIDTransaction := orderStatus.GetTransactionStatus()[0].GetClOrderId()
							switch TransactionStatus_Status_name[int32(orderStatus.GetTransactionStatus()[0].GetStatus())] {
							case TransactionStatus_REJECTED.String():
								//noq := newOrderList[j]
								if noq, ok := newOrderMap.Load(clorIDTransaction); ok {
									noq, _ := noq.(NewOrderCancelUpdateStatus)
									noq.status = "rejected"
									noq.reason = orderStatus.GetTransactionStatus()[0].GetTextMessage()
									noq.channel <- noq
									newOrderMap.Delete(clorIDTransaction)
									break
								}
							case TransactionStatus_ACK_PLACE.String():
								clorIDOrder := orderStatus.GetOrder().GetClOrderId()
								if noq, ok := newOrderMap.Load(clorIDOrder); ok {
									noq, _ := noq.(NewOrderCancelUpdateStatus)
									noq.status = "ok"
									noq.channel <- noq
									newOrderMap.Delete(clorIDOrder)
								}
								//Update Working orders
								accountID := orderStatus.GetOrder().GetAccountId()

								var wo WorkingOrder
								wo.orderID = orderStatus.GetOrderId()
								wo.chainOrderID = orderStatus.GetChainOrderId()
								wo.clorID = orderStatus.GetOrder().GetClOrderId()
								wo.contractID = orderStatus.GetOrder().GetContractId()
								wo.quantity = orderStatus.GetRemainingQty()
								wo.ordType = orderStatus.GetOrder().GetOrderType()
								wo.timeInForce = orderStatus.GetOrder().GetDuration()
								wo.text = "WORKING"

								var price int32
								if (wo.ordType == 2 || wo.ordType == 4 ) {
									price = orderStatus.GetOrder().GetLimitPrice()
								} else if (wo.ordType == 3 || wo.ordType == 4 ) {
									price = orderStatus.GetOrder().GetStopPrice()
								}
								wo.sideNum = orderStatus.GetOrder().GetSide()
								wo.side = Order_Side_name[int32(wo.sideNum)]

								if metadata, ok := metadataMap[wo.contractID]; ok {
									wo.symbol = metadata.GetTitle()
									wo.productDescription = metadata.GetDescription()
									wo.priceScale = metadata.GetCorrectPriceScale()
									wo.price = float64(price) * metadata.GetCorrectPriceScale()
								}

								if user, ok := userMap.getUser(accountID); ok {
									//fmt.Println("append new order to working")
									user.workingOrderMap.Store(wo.chainOrderID, &wo)
								}

							case TransactionStatus_FILL.String():
								accountID := orderStatus.GetOrder().GetAccountId()
								chainOrderID := orderStatus.GetChainOrderId()
								if user, ok := userMap.getUser(accountID); ok {
									if wo, ok := user.workingOrderMap.Load(chainOrderID); ok {
										wo, _ := wo.(*WorkingOrder)
										wo.quantity = orderStatus.GetRemainingQty()
										//fmt.Printf("%d left \n",wo.quantity)
										if wo.quantity == 0 {
											user.workingOrderMap.Delete(chainOrderID)
										}
									}
								}
								//fillQuantity := orderStatus.GetFillQty()
								//fillPrice := orderStatus.GetAvgFillPrice()
								// Notification here

							case TransactionStatus_ACK_CANCEL.String():
								if coq, ok := cancelOrderMap.Load(clorIDTransaction); ok {
									coq, _ := coq.(NewOrderCancelUpdateStatus)
									coq.status = "ok"
									coq.channel <- coq
									cancelOrderMap.Delete(clorIDTransaction)

								}
								//Update Working orders
								chainOrderID := orderStatus.GetChainOrderId()
								accountID := orderStatus.GetOrder().GetAccountId()
								if user, ok := userMap.getUser(accountID); ok {
									if _, ok := user.workingOrderMap.Load(chainOrderID); ok {
										user.workingOrderMap.Delete(chainOrderID)
										break
									}
								}
							case TransactionStatus_REJECT_CANCEL.String():
								if coq, ok := cancelOrderMap.Load(clorIDTransaction); ok {
									coq, _ := coq.(NewOrderCancelUpdateStatus)
									coq.status = "rejected"
									coq.reason = orderStatus.GetTransactionStatus()[0].GetTextMessage()
									coq.channel <- coq
									cancelOrderMap.Delete(clorIDTransaction)

								}
							case TransactionStatus_ACK_MODIFY.String():
								clorIDOrder := orderStatus.GetOrder().GetClOrderId()
								if moq, ok := updateOrderMap.Load(clorIDOrder); ok {
									moq, _ := moq.(NewOrderCancelUpdateStatus)
									moq.status = "ok"
									moq.channel <- moq
									updateOrderMap.Delete(clorIDOrder)
								}
								//update working order
								//account := orderStatus.GetEnteredByUser()
								accountID := orderStatus.GetOrder().GetAccountId()
								chainOrderID := orderStatus.GetChainOrderId()
								if user, ok := userMap.getUser(accountID); ok {
									if wo, ok := user.workingOrderMap.Load(chainOrderID); ok {
										wo, _ := wo.(*WorkingOrder)
										wo.orderID = orderStatus.GetOrderId()
										wo.clorID = orderStatus.GetOrder().GetClOrderId()
										wo.quantity = orderStatus.GetRemainingQty()
										wo.timeInForce = orderStatus.GetOrder().GetDuration()
										ordType := orderStatus.GetOrder().GetOrderType()

										if (ordType == 2 || wo.ordType == 4) {
											wo.price = float64(orderStatus.GetOrder().GetLimitPrice()) * wo.priceScale
										} else if (wo.ordType == 3 || wo.ordType == 4 ) {
											wo.price = float64(orderStatus.GetOrder().GetStopPrice()) * wo.priceScale
										}
									}
								}
							case TransactionStatus_REJECT_MODIFY.String():
								if moq, ok := updateOrderMap.Load(clorIDTransaction); ok {
									moq, _ := moq.(NewOrderCancelUpdateStatus)
									moq.status = "rejected"
									moq.reason = orderStatus.GetTransactionStatus()[0].GetTextMessage()
									moq.channel <- moq
									updateOrderMap.Delete(clorIDTransaction)
								}
							case TransactionStatus_ACTIVEAT.String():
								clorIDOrder := orderStatus.GetOrder().GetClOrderId()
								if noq, ok := newOrderMap.Load(clorIDOrder); ok {
									noq, _ := noq.(NewOrderCancelUpdateStatus)
									noq.status = "ok"
									noq.channel <- noq
									newOrderMap.Delete(clorIDOrder)
								}
								//Update Working orders
								accountID := orderStatus.GetOrder().GetAccountId()

								var wo WorkingOrder
								wo.orderID = orderStatus.GetOrderId()
								wo.chainOrderID = orderStatus.GetChainOrderId()
								wo.clorID = orderStatus.GetOrder().GetClOrderId()
								wo.contractID = orderStatus.GetOrder().GetContractId()
								wo.quantity = orderStatus.GetRemainingQty()
								wo.ordType = orderStatus.GetOrder().GetOrderType()
								wo.timeInForce = orderStatus.GetOrder().GetDuration()
								wo.text = "ACTIVEAT"

								var price int32
								if (wo.ordType == 2 || wo.ordType == 4 ) {
									price = orderStatus.GetOrder().GetLimitPrice()
								} else if (wo.ordType == 3 || wo.ordType == 4 ) {
									price = orderStatus.GetOrder().GetStopPrice()
								}
								wo.sideNum = orderStatus.GetOrder().GetSide()
								wo.side = Order_Side_name[int32(wo.sideNum)]

								if metadata, ok := metadataMap[wo.contractID]; ok {
									wo.symbol = metadata.GetTitle()
									wo.productDescription = metadata.GetDescription()
									wo.priceScale = metadata.GetCorrectPriceScale()
									wo.price = float64(price) * metadata.GetCorrectPriceScale()
								}

								if user, ok := userMap.getUser(accountID); ok {
									//fmt.Println("append new order to working")
									user.workingOrderMap.Store(wo.chainOrderID, &wo)
								}
							}
						} else { //Trade subscription snapshot
							if orderStatus.GetStatus() == 3 || orderStatus.GetStatus() == 11 {
								//fmt.Println("in order status snapshot")
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

								if metadata, ok := metadataMap[wo.contractID]; ok {
									wo.symbol = metadata.GetTitle()
									wo.productDescription = metadata.GetDescription()
									wo.priceScale = metadata.GetCorrectPriceScale()
									wo.price = float64(price) * wo.priceScale
								}

								if user, ok := userMap.getUser(accountID); ok {
									user.workingOrderMap.Store(wo.chainOrderID, &wo)
								}
							}
						}
					}
				case "collateral_status":
					for _, col := range msg.GetCollateralStatus() {
						accountID := col.GetAccountId()
						if user, ok := userMap.getUser(accountID); ok {
							user.collateralInfo.currency = col.GetCurrency()
							user.collateralInfo.marginCredit = col.GetMarginCredit()
							user.collateralInfo.mvf = col.GetMvf()
							user.collateralInfo.mvo = col.GetMvo()
							user.collateralInfo.upl = col.GetOte()
							user.collateralInfo.purchasingPower = col.GetPurchasingPower()
							user.collateralInfo.totalMargin = col.GetTotalMargin()
						}
					}

				case "trade_subscription_status":
				case "trade_snapshot_completion":
					for _, tsc := range msg.GetTradeSnapshotCompletion() {
						for _, scope := range tsc.GetSubscriptionScope() {
							switch int(scope) {
							case 1:
								cqgAccountMap.accountMap[username].chanOrderSubscription <- msg
							case 2:
								cqgAccountMap.accountMap[username].chanPositionSubcription <- msg
							case 3:
								cqgAccountMap.accountMap[username].chanCollateralSubscription <- msg
							}
						}
					}
				case "user_message":
					msgType := msg.GetUserMessage()[0].GetMessageType()
					if msgType == 1 {
						//*********** CRITICAL ERROR ALERT ADMIN IMMEDIATELY
						fmt.Println("*********** CRITICAL ERROR ALERT ADMIN IMMEDIATELY *******")
						fmt.Println("Try logoff and logon again")
						recoverFromDisconnection(connWithLock, username)
						break Loop
					}
				}
			}
		}
	}
}
func RecvMessageOne(connWithLock ConnWithLock) (msg *ServerMsg) {

	connWithLock.rwmux.RLock()
	_, message, err := connWithLock.conn.ReadMessage()
	if err != nil {
		fmt.Println("CQG read error:", err)
		log.Println("CQG read error:", err)
		connWithLock.status = "down"
		return nil
	}
	msg = &ServerMsg{}
	proto.Unmarshal(message, msg)
	//fmt.Printf("recv: %s \n", msg)
	log.Printf("recv: %s \n", msg)
	connWithLock.rwmux.RUnlock()
	return msg
}

func recoverFromDisconnection(connWithLock ConnWithLock, username string) {
	account := cqgAccountMap.accountMap[username]
	cqgAccountMap.removeAccount(username)
	var err bool

	for {
		for key, _ := range account.userMap {
			fmt.Printf("Reconnecting with %s pass: %s  account: %d\n", account.username, account.password, key)
			result := CQG_StartWebApi(account.username, account.password, key)
			if result == -1 {
				err = true
			} else {
				delete(account.userMap, key)
			}
		}
		if err {
			err = false
			time.Sleep(20 * time.Second)
		} else {
			break
		}
	}
	fmt.Println("break loop in recovering")

}
func checkUserLogonStatus(accountID int32) int {
	if user, ok := userMap.getUser(accountID); ok {
		if _, ok := cqgAccountMap.accountMap[user.username]; !ok {
			return -1
		}
	} else {
		return -1
	}
	return 0
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
func CQG_InitiateLogging() {
	f, err := os.OpenFile("cqglog.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("error open log file for CQG")
	}
	log.SetOutput(f)
}

type CQGAccountMap struct {
	mux        sync.Mutex
	accountMap map[string]*CQGAccount
}

func NewCQGAccountMap() *CQGAccountMap {
	return &CQGAccountMap{accountMap: make(map[string]*CQGAccount)}
}
func (cam CQGAccountMap) addAccount(account *CQGAccount) {
	cam.mux.Lock()
	cam.accountMap[account.username] = account
	cam.mux.Unlock()
}
func (cam CQGAccountMap) removeAccount(username string) {
	cam.mux.Lock()
	delete(cam.accountMap, username)
	cam.mux.Unlock()
}

type CQGAccount struct {
	username                   string
	password                   string
	metadataMap                map[uint32]*ContractMetadata
	userMap                    map[int32]*User
	mux                        sync.Mutex
	connWithLock               ConnWithLock
	chanLogon                  chan *ServerMsg
	chanOrderSubscription      chan *ServerMsg
	chanPositionSubcription    chan *ServerMsg
	chanCollateralSubscription chan *ServerMsg
}

func NewCQGAccount() *CQGAccount {
	return &CQGAccount{userMap:     make(map[int32]*User), metadataMap: make(map[uint32]*ContractMetadata),
		chanLogon:                  make(chan *ServerMsg), chanOrderSubscription: make(chan *ServerMsg),
		chanCollateralSubscription: make(chan *ServerMsg),
		chanPositionSubcription:    make(chan *ServerMsg),
	}
}
func (cqgAccount *CQGAccount) addUser(user *User) {
	cqgAccount.mux.Lock()
	cqgAccount.userMap[user.accountID] = user
	cqgAccount.mux.Unlock()
}

type UserMap struct {
	userMap map[int32]*User
	mux     sync.Mutex
}

func NewUserMap() *UserMap {
	return &UserMap{userMap: make(map[int32]*User)}
}
func (up *UserMap) addUser(user *User) {
	up.mux.Lock()
	up.userMap[user.accountID] = user
	up.mux.Unlock()
}
func (up *UserMap) getUser(accountID int32) (*User, bool) {
	u, e := up.userMap[accountID]
	return u, e
}

type User struct {
	username        string
	accountID       int32
	workingOrderMap syncmap.Map
	positionMap     syncmap.Map
	collateralInfo  CollateralInfo
}

func (user User) getMetadataMap(accountID int32, id uint32) *ContractMetadata {
	u, _ := userMap.getUser(accountID)
	cqgAccount := cqgAccountMap.accountMap[u.username]
	return cqgAccount.metadataMap[id]
}

type ConnWithLock struct {
	rwmux  sync.RWMutex
	conn   *websocket.Conn
	status string
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
	text         string

	symbol             string
	productDescription string
	priceScale         float64
}
type Position struct {
	quantity           uint32
	side               string
	shortBool          bool
	price              float64
	contractID         uint32
	symbol             string
	productDescription string
	priceScale         float64
	tickValue          float64
	subPositionMap     map[int32]*OpenPosition
}

func (position *Position) updatePriceAndQty() {
	position.quantity = 0
	position.price = 0
	for _, openPosition := range position.subPositionMap {
		position.quantity += openPosition.GetQty()
		position.price += openPosition.GetPrice() * float64(openPosition.GetQty())
	}
	//averaging out the price
	position.price /= float64(position.quantity)

}

type CollateralInfo struct {
	currency        string
	totalMargin     float64
	purchasingPower float64
	upl             float64
	mvo             float64
	marginCredit    float64
	mvf             float64
}
type NewOrderCancelUpdateStatus struct {
	clorderID string
	status    string
	reason    string
	channel   chan NewOrderCancelUpdateStatus
}

type InformationRequestStatus struct {
	id         uint32
	username   string
	contractID uint32
	status     string
	reason     string
	channel    chan InformationRequestStatus
}

type UserInfoRequest struct {
	username         string
	accountID        int32
	workingOrderList []WorkingOrder
	positionList     []Position
	collateralInfo   CollateralInfo
	status           string
	reason           string
}
