package main

import (
	"os"
	"bufio"
	"fmt"
	"github.com/rs/xid"
	"reflect"

	"os/exec"
)

var err error

func main() {

	CQG_InitiateLogging() //Initiate logging , call once when the server starts
	//Start CQG for this user
	//Call this function for each user who want to use CQG
	accountList := []int32{16958204}
	result := CQG_StartWebApi("VTechapi","pass" ,accountList)
	if result == -1 {
		fmt.Println("fail to initiate CQG API for account VTechApi")
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
			uir := CQG_GetPosition( 16958204)
			if uir != nil && uir.Status == "ok"{
				for _, po := range uir.PositionList {
					fmt.Printf("%s %d %s at %f \n", po.side, po.quantity, po.symbol, po.price)
				}
				fmt.Printf("Your total unrealized P&L is: %f %s\n", uir.CollateralInfo.Upl, uir.CollateralInfo.Currency)
			} else{
				fmt.Println("Error getting P&L: ",uir.Reason)
			}

		case "2":
			ordStatus := CQG_NewOrderRequest(1, 16958204, "ENQU7", xid.New().String(), 2, 5800, 2, 1, 5, false, makeTimestamp())
			if (ordStatus.Status == "ok") {
				fmt.Println("Order Placed Successfully")
			} else if (ordStatus.Status == "rejected") {
				fmt.Printf("Order Rejected \n")
				if (ordStatus.Reason != "") {
					fmt.Printf("Reason: %s", ordStatus.Reason)
				}
			}
		case "3":
			fmt.Println("Working Order:")
			uir := CQG_GetWorkingOrder(16958204) //return a list of working order
			if uir != nil && uir.Status == "ok" {
				for _, wo := range uir.WorkingOrderList {
					fmt.Printf("%s %d %s at %f \n", wo.side, wo.quantity, wo.symbol, wo.price)
				}
			} else{
				fmt.Println("Error getting working order")
			}
		case "4":
			fmt.Println("The answer is Tri Do :3")
		case "5":
			uir := CQG_GetWorkingOrder(16958204) //return a list of working order
			if uir != nil && len(uir.WorkingOrderList) > 0 && uir.Status == "ok" {
				wo := uir.WorkingOrderList[0]
				ordStatus := CQG_CancelOrderRequest(1, wo.orderID, uir.AccountID, wo.clorID, xid.New().String(), makeTimestamp())
				if ordStatus.Status == "ok" {
					fmt.Println("Order Cancelled Successfully")
				} else if ordStatus.Status == "rejected" {
					fmt.Printf("Order cancel Rejected \n")
					if ordStatus.Reason != "" {
						fmt.Printf("Reason: %s", ordStatus.Reason)
					}
				}
			}
		case "6":
			uir := CQG_GetWorkingOrder(16958204) //return a list of working order
			if uir != nil && uir.Status == "ok"{
				for _, wo := range uir.WorkingOrderList {
					ordStatus := CQG_CancelOrderRequest(1, wo.orderID, uir.AccountID, wo.clorID, xid.New().String(), makeTimestamp())
					if (ordStatus.Status == "ok") {
						fmt.Println("Order Cancelled Successfully")
					} else if (ordStatus.Status == "rejected") {
						fmt.Printf("Order cancel Rejected \n")
						if (ordStatus.Reason != "") {
							fmt.Printf("Reason: %s", ordStatus.Reason)
						}
					}
				}

			}
		case "7":
			uir := CQG_GetWorkingOrder( 16958204) //return a list of working order
			if uir != nil && len(uir.WorkingOrderList) > 0 && uir.Status == "ok" {
				wo := uir.WorkingOrderList[0]
				fmt.Println(int32(wo.price / wo.priceScale))
				fmt.Println(wo.priceScale)
				ordStatus := CQG_UpdateOrderRequest(1, wo.orderID, uir.AccountID, wo.clorID, xid.New().String(), makeTimestamp(), 17, int32((wo.price-100) / wo.priceScale), 0, wo.timeInForce)
				if (ordStatus.Status == "ok") {
					fmt.Println("Order Updated Successfully")
				} else if (ordStatus.Status == "rejected") {
					fmt.Printf("Order update Rejected \n")
					if (ordStatus.Reason != "") {
						fmt.Printf("Reason: %s", ordStatus.Reason)
					}
				}
			}
		case "8":
			uir := CQG_GetCollateralInfo(16958204)
			if uir != nil {
				o := reflect.TypeOf(uir.CollateralInfo)
				v := reflect.ValueOf(uir.CollateralInfo)
				for i:=0 ; i< v.NumField(); i++{
					fmt.Printf("%s :",o.Field(i).Name)
					fmt.Println(v.Field(i))
				}
			}
		case "9":
			break Loop
		default:
			cmd := exec.Command("clear") //Linux example, its tested
			cmd.Stdout = os.Stdout
			cmd.Run()
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
