package cqgwebapi

import (
	"os"
	"bufio"
	"fmt"
	"github.com/rs/xid"
	"reflect"

	"os/exec"
	"strconv"
)

var err error

func main() {

	CQG_InitiateLogging() //Initiate logging , call once when the server starts
	//Start CQG for this user
	//Call this function for each user who want to use CQG
	accountList := []int32{16958204}
	result := CQG_StartWebApi("VTechapi", "pass", accountList)
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
			fmt.Print("Position :")
			uir := CQG_GetPosition(16958204)
			if uir != nil && uir.Status == "ok" {
				for _, po := range uir.PositionList {
					fmt.Printf("%s %d %s at %f \n", po.Side, po.Quantity, po.Symbol, po.Price)
				}
				fmt.Printf("Your total unrealized P&L is: %f %s\n", uir.CollateralInfo.Upl, uir.CollateralInfo.Currency)
			} else {
				fmt.Println("Error getting P&L: ", uir.Reason)
			}

		case "2":
			scanner := bufio.NewScanner(os.Stdin)
			fmt.Print("Enter symbol:")
			scanner.Scan()
			symbol := scanner.Text()

			fmt.Print("Enter quantity:")
			scanner.Scan()
			qty, _ := strconv.ParseInt(scanner.Text(), 10, 64)

			fmt.Print("Enter Side 1 or 2:")
			scanner.Scan()
			side, _ := strconv.ParseInt(scanner.Text(), 10, 64)

			fmt.Print("Enter Ordertype:")
			scanner.Scan()
			orderType, _ := strconv.ParseInt(scanner.Text(), 10, 64)

			fmt.Print("Enter LimitPrice:")
			scanner.Scan()
			limitPrice, _ := strconv.ParseFloat(scanner.Text(), 64)

			fmt.Print("Enter Stop Price:")
			scanner.Scan()
			stopPrice, _ := strconv.ParseFloat(scanner.Text(), 64)

			fmt.Print("Enter Duration:")
			scanner.Scan()
			duration, _ := strconv.ParseInt(scanner.Text(), 10, 64)

			ordStatus := CQG_NewOrderRequest(1, 16958204, symbol, xid.New().String(), uint32(orderType), limitPrice, stopPrice, uint32(duration), uint32(side), uint32(qty), false, makeTimestamp())
			if ordStatus.Status == "ok" {
				fmt.Println("Order Placed Successfully")
				fmt.Printf("Your order ID is %s", ordStatus.OrderID)
			} else if ordStatus.Status == "rejected" {
				fmt.Printf("Order Rejected \n")
				if ordStatus.Reason != "" {
					fmt.Printf("Reason: %s", ordStatus.Reason)
				}
			}
		case "3":
			fmt.Println("Working Order:")
			uir := CQG_GetWorkingOrder(16958204) //return a list of working order
			if uir != nil && uir.Status == "ok" {
				for _, wo := range uir.WorkingOrderList {
					fmt.Printf("%s %s %d %s at %f \n", wo.OrderID, wo.Side, wo.Quantity, wo.Symbol, wo.Price)
				}
			} else {
				fmt.Println("Error getting working order")
			}
		case "4":
			scanner := bufio.NewScanner(os.Stdin)
			fmt.Print("Enter symbol:")
			scanner.Scan()
			symbol := scanner.Text()

			fmt.Print("Enter subscribe status true/false:")
			scanner.Scan()
			sub := scanner.Text()

			var subscribe bool
			if sub == "true"{
				subscribe = true
			}else{
				subscribe = false
			}

			ifr := CQG_InformationRequest(symbol, 1, "VTechapi", subscribe)
			if ifr.Status == "ok" {
				fmt.Println("infomration request sent succesfully")
			}
		case "5":
			scanner := bufio.NewScanner(os.Stdin)
			fmt.Print("Enter orderId to cancel:")
			scanner.Scan()
			orderId := scanner.Text()

			uir := CQG_GetWorkingOrder(16958204) //return a list of working order
			if uir != nil && len(uir.WorkingOrderList) > 0 && uir.Status == "ok" {
				for _, wo := range uir.WorkingOrderList {
					if wo.OrderID == orderId {
						ordStatus := CQG_CancelOrderRequest(1, wo.OrderID, uir.AccountID, wo.ClorID, xid.New().String(), makeTimestamp())
						if ordStatus.Status == "ok" {
							fmt.Println("Order Cancelled Successfully")
						} else if ordStatus.Status == "rejected" {
							fmt.Printf("Order cancel Rejected \n")
							if ordStatus.Reason != "" {
								fmt.Printf("Reason: %s", ordStatus.Reason)
							}
						}
						break
					}
				}

			}
		case "6":
			uir := CQG_GetWorkingOrder(16958204) //return a list of working order
			if uir != nil && uir.Status == "ok" {
				for _, wo := range uir.WorkingOrderList {
					ordStatus := CQG_CancelOrderRequest(1, wo.OrderID, uir.AccountID, wo.ClorID, xid.New().String(), makeTimestamp())
					if ordStatus.Status == "ok" {
						fmt.Println("Order Cancelled Successfully")
					} else if ordStatus.Status == "rejected" {
						fmt.Printf("Order cancel Rejected \n")
						if ordStatus.Reason != "" {
							fmt.Printf("Reason: %s", ordStatus.Reason)
						}
					}
				}

			}
		case "7":
			scanner := bufio.NewScanner(os.Stdin)
			fmt.Print("Enter orderId to modify:")
			scanner.Scan()
			orderId := scanner.Text()

			fmt.Print("Enter new qty or 0:")
			scanner.Scan()
			qty, _ := strconv.ParseInt(scanner.Text(), 10, 64)

			fmt.Print("Enter new limit price:")
			scanner.Scan()
			limitPrice, _ := strconv.ParseFloat(scanner.Text(), 64)

			fmt.Print("Enter new stop price:")
			scanner.Scan()
			stopPrice, _ := strconv.ParseFloat(scanner.Text(), 64)

			uir := CQG_GetWorkingOrder(16958204) //return a list of working order
			if uir != nil && len(uir.WorkingOrderList) > 0 && uir.Status == "ok" {
				for _, wo := range uir.WorkingOrderList {
					if wo.OrderID == orderId {
						//fmt.Println(int32(wo.Price / wo.PriceScale))
						//fmt.Println(wo.PriceScale)
						//change quantity and price , pass 0 or the same if doesnt want to change
						// remember to divide price to price scale before pass as param
						ordStatus := CQG_UpdateOrderRequest(1, wo.OrderID, uir.AccountID, wo.ClorID, xid.New().String(), makeTimestamp(), uint32(qty), int32((limitPrice)/wo.PriceScale), int32((stopPrice)/wo.PriceScale), 0)
						if ordStatus.Status == "ok" {
							fmt.Println("Order Updated Successfully")
						} else if (ordStatus.Status == "rejected") {
							fmt.Printf("Order update Rejected \n")
							if (ordStatus.Reason != "") {
								fmt.Printf("Reason: %s", ordStatus.Reason)
							}
						}

						break
					}
				}
			}

		case "8":
			uir := CQG_GetCollateralInfo(16958204)
			if uir != nil {
				o := reflect.TypeOf(uir.CollateralInfo)
				v := reflect.ValueOf(uir.CollateralInfo)
				for i := 0; i < v.NumField(); i++ {
					fmt.Printf("%s :", o.Field(i).Name)
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
	fmt.Println("5) Cancel 1 working order")
	fmt.Println("6) Cancel All working order")
	fmt.Println("7) Replace First working order")
	fmt.Println("8) Full Colleteral Status")
	fmt.Println("9) Quit")
	fmt.Print("Action: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return scanner.Text(), scanner.Err()
}
