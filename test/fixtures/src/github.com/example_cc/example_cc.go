/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
)

var logger = shim.NewLogger("example_cc0")

// SimpleChaincode example simple Chaincode implementation
type SimpleChaincode struct {
}

type transaction struct {
	ObjectType string `json:"docType"` //docType is used to distinguish the various types of objects in state database
	TxType     string `json:"txType"`
	Amount     int    `json:"amount"`
	Message    string `json:"message"`
}

type history []transaction

type customer struct {
	ObjectType    string  `json:"docType"` //docType is used to distinguish the various types of objects in state database
	Name          string  `json:"name"`
	AccountNumber string  `json:"accountNumber"`
	PhoneNumber   string  `json:"phoneNumber"`
	Balance       int     `json:"balance"`
	History       history `json:"history"`
}

// ===============================================
// readCustomer - read a customer from chaincode state
// ===============================================
func (t *SimpleChaincode) readCustomer(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var name, jsonResp string
	var err error

	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting name of the customer to query")
	}

	name = args[0]
	valAsbytes, err := stub.GetState(name) //get the customer from chaincode state
	if err != nil {
		jsonResp = "{\"Error\":\"Failed to get state for " + name + "\"}"
		return shim.Error(jsonResp)
	} else if valAsbytes == nil {
		jsonResp = "{\"Error\":\"Marble does not exist: " + name + "\"}"
		return shim.Error(jsonResp)
	}

	return shim.Success(valAsbytes)
}

func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface) pb.Response {
	logger.Info("########### example_cc0 Init ###########")

	_, args := stub.GetFunctionAndParameters()
	var A, B string    // Entities
	var Aval, Bval int // Asset holdings
	var err error

	// Initialize the chaincode
	A = args[0]
	Aval, err = strconv.Atoi(args[1])
	if err != nil {
		return shim.Error("Expecting integer value for asset holding")
	}
	B = args[2]
	Bval, err = strconv.Atoi(args[3])
	if err != nil {
		return shim.Error("Expecting integer value for asset holding")
	}
	logger.Info("Aval = %d, Bval = %d\n", Aval, Bval)

	// Write the state to the ledger
	err = stub.PutState(A, []byte(strconv.Itoa(Aval)))
	if err != nil {
		return shim.Error(err.Error())
	}

	err = stub.PutState(B, []byte(strconv.Itoa(Bval)))
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(nil)

}

// Transaction makes payment of X units from A to B
func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	logger.Info("########### example_cc0 Invoke ###########")

	function, args := stub.GetFunctionAndParameters()

	if function == "delete" {
		// Deletes an entity from its state
		return t.delete(stub, args)
	}

	if function == "query" {
		// queries an entity state
		return t.query(stub, args)
	}
	if function == "move" {
		// Deletes an entity from its state
		return t.move(stub, args)
	}
	if function == "transfer" {
		return t.transfer(stub, args)
	}

	logger.Errorf("Unknown action, check the first argument, must be one of 'delete', 'query', or 'move'. But got: %v", args[0])
	return shim.Error(fmt.Sprintf("Unknown action, check the first argument, must be one of 'delete', 'query', or 'move'. But got: %v", args[0]))
}

func (t *SimpleChaincode) initCustomer(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var err error
	var balanceVal int

	//    0           1               2            3
	// "name", "accountNumber", "phoneNumber", "balance"
	if len(args) != 4 {
		return shim.Error("Incorrect number of arguments. Expecting 4")
	}

	// ==== Input sanitation ====
	fmt.Println("- start init customer")
	if len(args[0]) <= 0 {
		return shim.Error("1st argument must be a non-empty string")
	}
	if len(args[1]) <= 0 {
		return shim.Error("2nd argument must be a non-empty string")
	}
	if len(args[2]) <= 0 {
		return shim.Error("3rd argument must be a non-empty string")
	}
	if len(args[3]) <= 0 {
		return shim.Error("4th argument must be a non-empty string")
	}

	name := args[0]
	accountNumber := args[1]
	phoneNumber := args[2]
	balance := args[3]
	history := []transaction{}

	// ==== Check if customer already exists ====
	customerAsBytes, err := stub.GetState(name)
	if err != nil {
		return shim.Error("Failed to get customer: " + err.Error())
	} else if customerAsBytes != nil {
		fmt.Println("This customer already exists: " + name)
		return shim.Error("This customer already exists: " + name)
	}

	balanceVal, _ = strconv.Atoi(balance)

	// ==== Create customer object and marshal to JSON ====
	objectType := "customer"
	customer := &customer{objectType, name, accountNumber, phoneNumber, balanceVal, history}
	customerJSONasBytes, err := json.Marshal(customer)
	if err != nil {
		return shim.Error(err.Error())
	}

	// === Save customer to state ===
	err = stub.PutState(name, customerJSONasBytes)
	if err != nil {
		return shim.Error(err.Error())
	}

	// ==== Customer saved. Return success ====
	fmt.Println("- end init customer")
	return shim.Success(nil)

}

func (t *SimpleChaincode) move(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	// must be an invoke
	var A, B string    // Entities
	var Aval, Bval int // Asset holdings
	var X int          // Transaction value
	var err error

	if len(args) != 3 {
		return shim.Error("Incorrect number of arguments. Expecting 4, function followed by 2 names and 1 value")
	}

	A = args[0]
	B = args[1]

	// Get the state from the ledger
	// TODO: will be nice to have a GetAllState call to ledger
	Avalbytes, err := stub.GetState(A)
	if err != nil {
		return shim.Error("Failed to get state")
	}
	if Avalbytes == nil {
		return shim.Error("Entity not found")
	}
	Aval, _ = strconv.Atoi(string(Avalbytes))

	Bvalbytes, err := stub.GetState(B)
	if err != nil {
		return shim.Error("Failed to get state")
	}
	if Bvalbytes == nil {
		return shim.Error("Entity not found")
	}
	Bval, _ = strconv.Atoi(string(Bvalbytes))

	// Perform the execution
	X, err = strconv.Atoi(args[2])
	if err != nil {
		return shim.Error("Invalid transaction amount, expecting a integer value")
	}
	Aval = Aval - X
	Bval = Bval + X
	logger.Infof("Aval = %d, Bval = %d\n", Aval, Bval)

	// Write the state back to the ledger
	err = stub.PutState(A, []byte(strconv.Itoa(Aval)))
	if err != nil {
		return shim.Error(err.Error())
	}

	err = stub.PutState(B, []byte(strconv.Itoa(Bval)))
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(nil)
}

func (t *SimpleChaincode) transfer(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	// must be an invoke
	var A, B, phoneNumber, msg string // Entities
	var Aval, Bval int                // Asset holdings
	var X int                         // Transaction value
	var err error

	if len(args) != 5 {
		return shim.Error("Incorrect number of arguments. Expecting 5, function followed by 2 names and 3 value")
	}

	A = args[0]
	B = args[1]

	phoneNumber = args[3]
	logger.Infof("Passed phoneNumber: %s\n", phoneNumber)

	msg = args[4]
	logger.Infof("Passed message: %s\n", msg)

	senderAsBytes, err := stub.GetState(A)
	if err != nil {
		return shim.Error("Failed to get sender: " + err.Error())
	} else if senderAsBytes != nil {
		fmt.Println("Sender was found: " + A)
	}

	recipientAsBytes, err := stub.GetState(B)
	if err != nil {
		return shim.Error("Failed to get recipient: " + err.Error())
	} else if recipientAsBytes != nil {
		fmt.Println("Recipient was found: " + B)
	}

	// sender check
	sender := customer{}
	err = json.Unmarshal(senderAsBytes, &sender) //unmarshal it aka JSON.parse()
	if err != nil {
		return shim.Error(err.Error())
	}

	// recipent check
	recipient := customer{}
	err = json.Unmarshal(recipientAsBytes, &recipient) //unmarshal it aka JSON.parse()
	if err != nil {
		return shim.Error(err.Error())
	}

	// confirm if phoneNumber is correct
	if phoneNumber != recipient.PhoneNumber {
		return shim.Error("PhoneNumber is incorrect: " + err.Error())
	}

	Aval = sender.Balance
	Bval = recipient.Balance

	// Perform the execution
	X, err = strconv.Atoi(args[2])
	if err != nil {
		return shim.Error("Invalid transaction amount, expecting a integer value")
	}
	Aval = Aval - X
	Bval = Bval + X
	logger.Infof("Aval = %d, Bval = %d\n", Aval, Bval)

	sender.Balance = Aval
	recipient.Balance = Bval

	// ObjectType           string `json:"docType"` //docType is used to distinguish the various types of objects in state database
	// SenderName           string `json:"senderName"`
	// RecipientName        string `json:"recipientName"`
	// RecipientPhoneNumber string `json:"recipientPhoneNumber"`
	// Amount               int    `json:"amount"`
	// Message              string `json:"message"`

	// TODO: transaction to history
	objectType := "transaction"
	transactionA := &transaction{objectType, "出金", X, msg}
	transactionB := &transaction{objectType, "入金", X, msg}

	sender.History = append(sender.History, *transactionA)
	recipient.History = append(recipient.History, *transactionB)

	senderJSONasBytes, _ := json.Marshal(sender)
	err = stub.PutState(sender.Name, senderJSONasBytes) //rewrite the sender
	if err != nil {
		return shim.Error(err.Error())
	}

	recipientJSONasBytes, _ := json.Marshal(recipient)
	err = stub.PutState(recipient.Name, recipientJSONasBytes) //rewrite the sender
	if err != nil {
		return shim.Error(err.Error())
	}

	// To return transaction result
	transactionJSONasBytes, _ := json.Marshal(transactionA)

	fmt.Println("- end transfer (success)")
	return shim.Success(transactionJSONasBytes)

}

// Deletes an entity from state
func (t *SimpleChaincode) delete(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}

	A := args[0]

	// Delete the key from the state in ledger
	err := stub.DelState(A)
	if err != nil {
		return shim.Error("Failed to delete state")
	}

	return shim.Success(nil)
}

// Query callback representing the query of a chaincode
func (t *SimpleChaincode) query(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	var A string // Entities
	var err error

	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting name of the person to query")
	}

	A = args[0]

	// Get the state from the ledger
	Avalbytes, err := stub.GetState(A)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for " + A + "\"}"
		return shim.Error(jsonResp)
	}

	if Avalbytes == nil {
		jsonResp := "{\"Error\":\"Nil amount for " + A + "\"}"
		return shim.Error(jsonResp)
	}

	jsonResp := "{\"Name\":\"" + A + "\",\"Amount\":\"" + string(Avalbytes) + "\"}"
	logger.Infof("Query Response:%s\n", jsonResp)
	return shim.Success(Avalbytes)
}

func main() {
	err := shim.Start(new(SimpleChaincode))
	if err != nil {
		logger.Errorf("Error starting Simple chaincode: %s", err)
	}
}
