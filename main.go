package main

import (
	"os"
	"bufio"
	"fmt"
	"github.com/rs/xid"
	"reflect"

)

var err error

func main() {
	//Start CQG for this user
	//Call this function for each user who want to use CQG
	result := CQG_StartWebApi("VTechapi","pass" ,16958204)
	if result == -1 {
		fmt.Println("fail to initiate CQG API for user ",16958204)
	}
Loop:
	for {
		action, err := QueryAction()
		if err != nil {
			break
		}
		switch action {
		case "1":
			fmt.Println("Position :")
			user := CQG_GetPosition( 16958204)
			if user != nil {
				for _, po := range user.positionList {
					fmt.Printf("%s %d %s at %f \n", po.side, po.quantity, po.symbol, po.price)
				}
				fmt.Printf("Your total unrealized P&L is: %f %s\n",user.collateralInfo.upl,user.collateralInfo.currency)
			}

		case "2":
			ordStatus := CQG_NewOrderRequest(1, 16958204, "BZ", xid.New().String(), 2, 4700, 2, 1, 1, false, makeTimestamp())
			if (ordStatus.status == "ok") {
				fmt.Println("Order Placed Successfully")
			} else if (ordStatus.status == "rejected") {
				fmt.Printf("Order Rejected \n")
				if (ordStatus.reason != "") {
					fmt.Printf("Reason: %s", ordStatus.reason)
				}
			}
		case "3":
			fmt.Println("Working Order:")
			user := CQG_GetWorkingOrder(16958204) //return a list of working order
			if user != nil {
				for _, wo := range user.workingOrderList {
					fmt.Printf("%s %d %s at %f \n", wo.side, wo.quantity, wo.symbol, wo.price)
				}
			}
		case "4":
			fmt.Println("The answer is Tri Do :3")
		case "5":
			user := CQG_GetWorkingOrder(16958204) //return a list of working order
			if user != nil && len(user.workingOrderList) > 0 {
				wo := user.workingOrderList[0]
				ordStatus := CQG_CancelOrderRequest(1, wo.orderID, user.accountID, wo.clorID, xid.New().String(), makeTimestamp())
				if ordStatus.status == "ok" {
					fmt.Println("Order Cancelled Successfully")
				} else if ordStatus.status == "rejected" {
					fmt.Printf("Order cancel Rejected \n")
					if ordStatus.reason != "" {
						fmt.Printf("Reason: %s", ordStatus.reason)
					}
				}
			}
		case "6":
			user := CQG_GetWorkingOrder(16958204) //return a list of working order
			if user != nil && len(user.workingOrderList) > 0 {
				for _, wo := range user.workingOrderList {
					ordStatus := CQG_CancelOrderRequest(1, wo.orderID, user.accountID, wo.clorID, xid.New().String(), makeTimestamp())
					if (ordStatus.status == "ok") {
						fmt.Println("Order Cancelled Successfully")
					} else if (ordStatus.status == "rejected") {
						fmt.Printf("Order cancel Rejected \n")
						if (ordStatus.reason != "") {
							fmt.Printf("Reason: %s", ordStatus.reason)
						}
					}
				}

			}
		case "7":
			user := CQG_GetWorkingOrder( 16958204) //return a list of working order
			if user != nil && len(user.workingOrderList) > 0 {
				wo := user.workingOrderList[0]
				fmt.Println(int32(wo.price / wo.priceScale))
				fmt.Println(wo.priceScale)
				ordStatus := CQG_UpdateOrderRequest(1, wo.orderID, user.accountID, wo.clorID, xid.New().String(), makeTimestamp(), 2, int32(wo.price / wo.priceScale), 0, wo.timeInForce)
				if (ordStatus.status == "ok") {
					fmt.Println("Order Updated Successfully")
				} else if (ordStatus.status == "rejected") {
					fmt.Printf("Order update Rejected \n")
					if (ordStatus.reason != "") {
						fmt.Printf("Reason: %s", ordStatus.reason)
					}
				}
			}
		case "8":
			user := CQG_GetCollateralInfo(16958204)
			if user != nil {
				o := reflect.TypeOf(user.collateralInfo)
				v := reflect.ValueOf(user.collateralInfo)
				for i:=0 ; i< v.NumField(); i++{
					fmt.Printf("%s :",o.Field(i).Name)
					fmt.Println(v.Field(i))
				}
			}
		case "9":
			break Loop
		}
	}
}

func QueryAction() (string, error) {
	fmt.Println()
	fmt.Println("1) Get P&L")
	fmt.Println("2) Place new Order")
	fmt.Println("3) Get Working Order")
	fmt.Println("4) To know who is the most awesome guy to date with")
	fmt.Println("5) Cancel First working order")
	fmt.Println("6) Cancel All working order")
	fmt.Println("7) Replace First working order")
	fmt.Println("8) Full Colleteral Status")
	fmt.Println("9) Quit")
	fmt.Print("Action: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return scanner.Text(), scanner.Err()
}
