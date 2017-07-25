package main

import (
	"github.com/golang/protobuf/proto"
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
)

var cqgAccountMap = NewCQGAccountMap()
var metadataMap = make(map[uint32]*ContractMetadata)
var userMap = NewUserMap()

var newOrderList []NewOrderCancelUpdateStatus
var cancelOrderList []NewOrderCancelUpdateStatus
var updateOrderList []NewOrderCancelUpdateStatus

var chanLogon = make(chan *ServerMsg)
var chanInformationReport = make(chan *ServerMsg)
var chanOrderSubscription = make(chan *ServerMsg)
var chanPositionSubcription = make(chan *ServerMsg)
var chanCollateralSubscription = make(chan *ServerMsg)

func CQG_StartWebApi(username string, password string, accountID int32) int {

	cqgAccount := NewCQGAccount()
	if _, ok := cqgAccountMap.accountMap[username]; !ok {
		err := startNewConnection(cqgAccount)

		if err == nil {
			return -1
		}
		if(cqgAccount.connWithLock.conn == nil){
			fmt.Println("conn still null")
		}else{
			fmt.Println("conn not null anymore")
		}
		cqgAccount.username = username
		cqgAccount.password = password
		cqgAccountMap.addAccount(cqgAccount)
		msg := CQG_SendLogonMessage(username, accountID, password, "WebApiTest", "java-client")
		if msg.LogonResult.GetResultCode() == 0 {
			fmt.Printf("Logon Successfully!!! Let's make America great again \n")
		} else {
			fmt.Printf("Logon failed !! It's Obama's fault \n")
			return -1
		}

	} else {
		cqgAccount = cqgAccountMap.accountMap[username]
	}

	var user User
	user.username = username
	user.accountID = accountID
	cqgAccount.addUser(&user)
	userMap.addUser(&user)
	CQG_OrderSubscription(hash(xid.New().String()), true,username)

	return 0
}
func startNewConnection(cqgAccount *CQGAccount) *CQGAccount {
	var addr = flag.String("addr", "demoapi.cqg.com:443", "http service address")
	flag.Parse()
	log.SetFlags(0)

	u := url.URL{Scheme: "wss", Host: *addr, Path: ""}
	log.Printf("connecting to %s", u.String())

	cqgAccount.connWithLock.conn, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
		return nil
	}
	go RecvMessage(cqgAccount.connWithLock)

	return cqgAccount
}
func CQG_NewOrderRequest(id uint32, accountID int32, contractID uint32, clorderID string, orderType uint32, price int32, duration uint32, side uint32, qty uint32, is_manual bool, utc int64) (ordStatus NewOrderCancelUpdateStatus) {
	var c = make(chan NewOrderCancelUpdateStatus)
	user,_ := userMap.getUser(accountID)
	NewOrderRequest(id,user.username ,accountID, contractID, clorderID, orderType, price, duration, side, qty, is_manual, utc, c)
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
	if cqgAccount, ok := cqgAccountMap.accountMap[account]; ok {
		if user, ok := cqgAccount.userMap[accountID]; ok {
			return user
		}
	}
	return nil
}
func CQG_GetWorkingOrder(account string, accountID int32) *User {
	if cqgAccount, ok := cqgAccountMap.accountMap[account]; ok {
		if user, ok := cqgAccount.userMap[accountID]; ok {
			return user
		}
	}
	return nil
}
func CQG_GetCollateralInfo(account string, accountID int32) *User {
	if cqgAccount, ok := cqgAccountMap.accountMap[account]; ok {
		if user, ok := cqgAccount.userMap[accountID]; ok {
			return user
		}
	}
	return nil
}
func CQG_CancelOrderRequest(id uint32, orderID string, accountID int32, oldClorID string, clorID string, utc int64) (ordStatus NewOrderCancelUpdateStatus) {
	var c = make(chan NewOrderCancelUpdateStatus)
	user,_ := userMap.getUser(accountID)
	CancelOrderRequest(id, orderID, user.username,accountID, oldClorID, clorID, utc, c)
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

	user,_ := userMap.getUser(accountID)// get the user associated
	var c = make(chan NewOrderCancelUpdateStatus)
	UpdateOrderRequest(id, orderID, user.username,accountID, oldClorID, clorID, utc, qty, limitPrice, stopPrice, duration, c)
	select {
	case ordStatus = <-c:
		return ordStatus
	case <-getTimeOutChan():
		ordStatus.status = "rejected"
		ordStatus.reason = "time out"
	}
	return ordStatus
}

func SendMessage(message *ClientMsg,connWithLock ConnWithLock) {
	connWithLock.rwmux.Lock()
	out, _ := proto.Marshal(message)
	err := connWithLock.conn.WriteMessage(websocket.BinaryMessage, []byte(out))
	if err != nil {
		fmt.Println("error:", err)
		return
	} else {
		fmt.Printf("send: %s\n", *message)
	}
	connWithLock.rwmux.Unlock()
}
func RecvMessage(connWithLock ConnWithLock) {
	for {

		msg := RecvMessageOne(connWithLock)
		if (msg == nil) {
			continue
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
					for _, position := range msg.GetPositionStatus() {
						//update position
						if position.GetIsSnapshot() == true { // Snapshot
							accountID := position.GetAccountId()
							if len(position.GetOpenPosition()) > 0 {
								var userPosition Position
								userPosition.subPositionMap = make(map[int32]*OpenPosition)
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
									userPosition.subPositionMap[openPosition.GetId()] = openPosition
								}
								//averaging out the price
								userPosition.price /= float64(userPosition.quantity)

								//add to Position list of user
								if user, ok := userMap.getUser(accountID); ok {
									fmt.Println("add 1 position ", userPosition.symbol, userPosition.contractID)
									user.positionList = append(user.positionList, userPosition)
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
									userPosition.shortBool = true
								} else {
									userPosition.side = "BUY"
									userPosition.shortBool = false
								}

								//add to Position list of user
								if user, ok := userMap.getUser(accountID); ok {
									match := false
									for positionIndex := range user.positionList {
										if user.positionList[positionIndex].contractID == userPosition.contractID { //COntract already exist
											for _, openPosition := range position.GetOpenPosition() { //Update open_position/subposition
												if oldOpenPosition, ok := user.positionList[positionIndex].subPositionMap[openPosition.GetId()]; ok {
													oldOpenPosition.Qty = openPosition.Qty
													oldOpenPosition.Price = openPosition.Price

												} else { // add new open_position
													user.positionList[positionIndex].subPositionMap[openPosition.GetId()] = openPosition
												}
											}
											user.positionList[positionIndex].updatePriceAndQty()
											user.positionList[positionIndex].side = userPosition.side
											match = true
											if user.positionList[positionIndex].quantity == 0 { //remove position if qty =0 square position
												user.positionList = append(user.positionList[:positionIndex], user.positionList[positionIndex+1:]...)
												break
											}

										}
									}
									if match == false { // new contract added to position
										for _, openPosition := range position.GetOpenPosition() {
											userPosition.quantity += openPosition.GetQty()
											userPosition.price += openPosition.GetPrice() * float64(openPosition.GetQty())
											userPosition.subPositionMap[openPosition.GetId()] = openPosition
										}
										//averaging out the price
										userPosition.price /= float64(userPosition.quantity)
										user.positionList = append(user.positionList, userPosition)
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

								var price int32
								if (wo.ordType == 2 || wo.ordType == 4 ) {
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
										wo.price = float64(price) * metadata.GetCorrectPriceScale()
									}
								}

								if user, ok := userMap.getUser(accountID); ok {
									//fmt.Println("append new order to working")
									user.workingOrderList = append(user.workingOrderList, wo)
								}

							case TransactionStatus_FILL.String():
								accountID := orderStatus.GetOrder().GetAccountId()
								chainOrderID := orderStatus.GetChainOrderId()
								if user, ok := userMap.getUser(accountID); ok {
									for i := range user.workingOrderList {
										wo := &user.workingOrderList[i]
										if wo.chainOrderID == chainOrderID {
											wo.quantity = orderStatus.GetRemainingQty()
											//fmt.Printf("%d left \n",wo.quantity)
											if wo.quantity == 0 {
												user.workingOrderList = append(user.workingOrderList[:i], user.workingOrderList[i+1:]...)
											}
										}
									}
								}
								//Update Working orders
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
								//Update Working orders
								chainOrderID := orderStatus.GetChainOrderId()
								accountID := orderStatus.GetOrder().GetAccountId()
								if user, ok := userMap.getUser(accountID); ok {
									for k := range user.workingOrderList {
										if user.workingOrderList[k].chainOrderID == chainOrderID {
											user.workingOrderList = append(user.workingOrderList[:k], user.workingOrderList[k+1:]...)
											break
										}
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
								//update working order
								//account := orderStatus.GetEnteredByUser()
								accountID := orderStatus.GetOrder().GetAccountId()
								chainOrderID := orderStatus.GetChainOrderId()
								if user, ok := userMap.getUser(accountID); ok {
									for k := range user.workingOrderList {
										wo := &user.workingOrderList[k]
										if wo.chainOrderID == chainOrderID {
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

								if user, ok := userMap.getUser(accountID); ok {
									user.workingOrderList = append(user.workingOrderList, wo)
								}
							}
						}
					}
				case "collateral_status":
					for _, col := range msg.GetCollateralStatus() {
						accountID := col.GetAccountId()
							if user,ok := userMap.getUser(accountID) ; ok {
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
								chanOrderSubscription <- msg
							case 2:
								chanPositionSubcription <- msg
							case 3:
								chanCollateralSubscription <- msg
							}
						}
					}
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
func RecvMessageOne(connWithLock ConnWithLock) (msg *ServerMsg) {
	connWithLock.rwmux.RLock()
	_, message, err := connWithLock.conn.ReadMessage()
	if err != nil {
		fmt.Println("read error:", err)
		return
	}
	msg = &ServerMsg{}
	proto.Unmarshal(message, msg)
	//fmt.Printf("recv: %s \n", msg)
	connWithLock.rwmux.RUnlock()
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
func flipSide(side string, shortBool bool) (s string, b bool) {
	switch side {
	case "BUY":
		s = "SELL"
	case "SELL":
		s = "BUY"
	}

	switch shortBool {
	case true:
		b = false
	case false:
		b = true
	}
	return s, b
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
func (cam CQGAccountMap) removeAccount(account *CQGAccount) {
	cam.mux.Lock()
	delete(cam.accountMap, account.username)
	cam.mux.Unlock()
}

type CQGAccount struct {
	username     string
	password     string
	userMap      map[int32]*User
	mux          sync.Mutex
	connWithLock ConnWithLock
}

func NewCQGAccount() *CQGAccount {
	return &CQGAccount{userMap: make(map[int32]*User)}
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
	username         string
	accountID        int32
	workingOrderList []WorkingOrder
	positionList     []Position
	collateralInfo   CollateralInfo
}
type ConnWithLock struct {
	rwmux sync.RWMutex
	conn  *websocket.Conn
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
