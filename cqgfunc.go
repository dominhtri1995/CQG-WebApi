package cqgwebapi

import (
	"fmt"
)

func CQG_OrderSubscription(id uint32, subscribe bool, username string) {
	var arr []uint32
	arr = append(arr, 1)
	arr = append(arr,2)
	arr = append(arr, 3)
	clientMsg := &ClientMsg{
		TradeSubscription: []*TradeSubscription{
			{
				Id:                &id,
				Subscribe:         &subscribe,
				SubscriptionScope: arr,
			},
		},
	}
	SendMessage(clientMsg, cqgAccountMap.accountMap[username].connWithLock)
	_,_,_= <- cqgAccountMap.accountMap[username].chanOrderSubscription, <-cqgAccountMap.accountMap[username].chanPositionSubcription , <-cqgAccountMap.accountMap[username].chanCollateralSubscription
	fmt.Println("Subscription done")
}
func NewOrderRequest(id uint32,username string, accountID int32, contractID uint32, clorderID string, orderType uint32, limitPrice int32, stopPrice int32, duration uint32, side uint32, qty uint32, is_manual bool, utc int64, c chan NewOrderCancelUpdateStatus) {

	var noq NewOrderCancelUpdateStatus
	noq.ClorderID = clorderID
	noq.channel = c
	newOrderMap.Store(clorderID,noq)

	clientMsg := &ClientMsg{
		OrderRequest: []*OrderRequest{
			{
				RequestId: &id,
				NewOrder: &NewOrder{
					Order: &Order{
						AccountId:   &accountID,
						WhenUtcTime: &utc,
						ContractId:  &contractID,
						ClOrderId:   &clorderID,
						OrderType:   &orderType,
						Duration:    &duration,
						Side:        &side,
						Qty:         &qty,
						IsManual:    &is_manual,
					},
				},
			},
		},
	}

	if orderType == 2 || orderType ==4 {
		clientMsg.GetOrderRequest()[0].GetNewOrder().GetOrder().LimitPrice = &limitPrice
	}
	if  orderType == 3 || orderType == 4 {
		clientMsg.GetOrderRequest()[0].GetNewOrder().GetOrder().StopPrice = &stopPrice
	}

	SendMessage(clientMsg, cqgAccountMap.accountMap[username].connWithLock)
}
func CancelOrderRequest(id uint32, orderID string, username string, accountID int32, oldClorID string, clorderID string, utc int64, c chan NewOrderCancelUpdateStatus) {
	var coq NewOrderCancelUpdateStatus
	coq.ClorderID = clorderID
	coq.channel = c
	cancelOrderMap.Store(clorderID,coq)

	clientMsg := &ClientMsg{
		OrderRequest: []*OrderRequest{
			{
				RequestId: &id,
				CancelOrder: &CancelOrder{
					OrderId:       &orderID,
					AccountId:     &accountID,
					OrigClOrderId: &oldClorID,
					ClOrderId:     &clorderID,
					WhenUtcTime:   &utc,
				},
			},
		},
	}
	SendMessage(clientMsg,cqgAccountMap.accountMap[username].connWithLock)
}
func UpdateOrderRequest(id uint32, orderID string,username string, accountID int32, oldClorderID string, clorderID string, utc int64,
	qty uint32, limitPrice int32, stopPrice int32, duration uint32,c chan NewOrderCancelUpdateStatus, ) {
	/* Pass in qty,limitprice, stopprice and duration if you want to change these elements.
	// Otherwise pass in 0
	 */
	var moq NewOrderCancelUpdateStatus
	moq.ClorderID = clorderID
	moq.channel = c
	updateOrderMap.Store(clorderID,moq)

	clientMsg := &ClientMsg{
		OrderRequest: []*OrderRequest{
			{
				RequestId: &id,
				ModifyOrder: &ModifyOrder{
					OrderId:       &orderID,
					AccountId:     &accountID,
					OrigClOrderId: &oldClorderID,
					ClOrderId:     &clorderID,
					WhenUtcTime:   &utc,
				},
			},
		},
	}
	if qty != 0 {
		clientMsg.GetOrderRequest()[0].GetModifyOrder().Qty = &qty
	}
	if limitPrice != 0 {
		clientMsg.GetOrderRequest()[0].GetModifyOrder().LimitPrice = &limitPrice
	}
	if stopPrice != 0 {
		clientMsg.GetOrderRequest()[0].GetModifyOrder().StopPrice = &stopPrice
	}
	if duration != 0 {
		clientMsg.GetOrderRequest()[0].GetModifyOrder().Duration = &duration
	}
	SendMessage(clientMsg,cqgAccountMap.accountMap[username].connWithLock)
}
func CQG_InformationRequest(symbol string, id uint32,username string, subscribe bool) InformationRequestStatus{

	var ifr InformationRequestStatus
	ifr.Id =id
	ifr.Username = username
	ifr.Status = "ok"
	ifr.channel = make (chan InformationRequestStatus)
	informationRequestMap.Store(id,ifr)

	clientMsg := &ClientMsg{
		InformationRequest: []*InformationRequest{
			{Id: &id,
				SymbolResolutionRequest: &SymbolResolutionRequest{
					Symbol: &symbol,
				},
				Subscribe: &subscribe,
			},
		},
	}
	SendMessage(clientMsg, cqgAccountMap.accountMap[username].connWithLock)
	select {
	case ifr = <-ifr.channel:
		informationRequestMap.Delete(id)
		return ifr
	case <- getTimeOutChan():
		ifr.Status ="rejected"
		ifr.Reason ="Unable to obtain Symbol from server"
		informationRequestMap.Delete(id)
	}
	return ifr
}
func CQG_SendLogonMessage(username string, password string, clientAppID string, clientVersion string) *ServerMsg {
	LogonMessage := &ClientMsg{
		Logon: &Logon{UserName: &username,
			Password:           &password,
			ClientAppId:        &clientAppID,
			ClientVersion:      &clientVersion},
	}

	SendMessage(LogonMessage,cqgAccountMap.accountMap[username].connWithLock)
	msg := <- cqgAccountMap.accountMap[username].chanLogon
	return msg
}
func CQG_SendLogoffMessage(reason string,username string){
	LogoffMessage := &ClientMsg{
		Logoff: &Logoff{
			TextMessage: &reason,
		},
	}
	SendMessage(LogoffMessage, cqgAccountMap.accountMap[username].connWithLock)
}
