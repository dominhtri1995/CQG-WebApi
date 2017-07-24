package main

import (
	"fmt"
	"github.com/rs/xid"
)

func CQG_OrderSubscription(id uint32, subscribe bool) {
	var arr []uint32
	arr = append(arr, 1)
	arr = append(arr,2)
	clientMsg := &ClientMsg{
		TradeSubscription: []*TradeSubscription{
			{
				Id:                &id,
				Subscribe:         &subscribe,
				SubscriptionScope: arr,
			},
		},
	}
	SendMessage(clientMsg)
	_ = <-chanOrderSubscription
}
func CQG_PositionSubscription(id uint32, subscribe bool) {
	var arr []uint32
	arr = append(arr, 2)
	clientMsg := &ClientMsg{
		TradeSubscription: []*TradeSubscription{
			{
				Id:                &id,
				Subscribe:         &subscribe,
				SubscriptionScope: arr,
			},
		},
	}
	SendMessage(clientMsg)
}
func NewOrderRequest(id uint32, accountID int32, contractID uint32, clorderID string, orderType uint32, price int32, duration uint32, side uint32, qty uint32, is_manual bool, utc int64, c chan NewOrderCancelUpdateStatus) {

	var noq NewOrderCancelUpdateStatus
	noq.clorderID = clorderID
	noq.channel = c
	newOrderList = append(newOrderList, noq)

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
		clientMsg.GetOrderRequest()[0].GetNewOrder().GetOrder().LimitPrice = &price
	} else if  orderType == 3 || orderType == 4 {
		clientMsg.GetOrderRequest()[0].GetNewOrder().GetOrder().StopPrice = &price
	}

	SendMessage(clientMsg)
}
func CancelOrderRequest(id uint32, orderID string, accountID int32, oldClorID string, clorderID string, utc int64, c chan NewOrderCancelUpdateStatus) {
	var coq NewOrderCancelUpdateStatus
	coq.clorderID = clorderID
	coq.channel = c
	cancelOrderList = append(cancelOrderList, coq)

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
	SendMessage(clientMsg)
}
func UpdateOrderRequest(id uint32, orderID string, accountID int32, oldClorderID string, clorderID string, utc int64,
	qty uint32, limitPrice int32, stopPrice int32, duration uint32,c chan NewOrderCancelUpdateStatus, ) {
	/* Pass in qty,limitprice, stopprice and duration if you want to change these elements.
	// Otherwise pass in 0
	 */
	var moq NewOrderCancelUpdateStatus
	moq.clorderID = clorderID
	moq.channel = c
	updateOrderList = append(updateOrderList, moq)

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
	if (qty != 0) {
		clientMsg.GetOrderRequest()[0].GetModifyOrder().Qty = &qty
	}
	if (limitPrice != 0) {
		clientMsg.GetOrderRequest()[0].GetModifyOrder().LimitPrice = &limitPrice
	}
	if (stopPrice != 0) {
		clientMsg.GetOrderRequest()[0].GetModifyOrder().StopPrice = &stopPrice
	}
	if (duration != 0) {
		clientMsg.GetOrderRequest()[0].GetModifyOrder().Qty = &duration
	}
	SendMessage(clientMsg)
}
func CQG_InformationRequest(symbol string, id uint32) {
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
	_ = <-chanInformationReport
}
func CQG_SendLogonMessage(username string, accountID int32, password string, clientAppID string, clientVersion string) {
	LogonMessage := &ClientMsg{
		Logon: &Logon{UserName: &username,
			Password:           &password,
			ClientAppId:        &clientAppID,
			ClientVersion:      &clientVersion},
	}

	SendMessage(LogonMessage)
	msg := <-chanLogon
	if msg.LogonResult.GetResultCode() == 0 {
		fmt.Printf("Logon Successfully!!! Let's make America great again \n")
		var user User
		user.username = username
		user.accountID= accountID
		userLogonList = append(userLogonList, user)

		CQG_OrderSubscription(hash(xid.New().String()), true)
	} else {
		fmt.Printf("Logon failed !! It's Obama's fault \n")
	}

}
