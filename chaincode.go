package main

import (
	"crypto/x509"
	"encoding/asn1"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
)

//==============================================================================================================================
//	 Structure Definitions
//==============================================================================================================================
//	SimpleChaincode - A blank struct for use with Shim (An IBM Blockchain included go file used for get/put state
//					  and other IBM Blockchain functions)
//==============================================================================================================================
type SimpleChaincode struct {
}

type ECertResponse struct {
	OK string `json:"OK"`
}

type Track struct {
	Id     			string 			`json:"id"`
	Title 			string 			`json:"title"`
	Artist			string 			`json:"artist"`
	Beneficiaries 	[]Beneficiary	`json:"beneficiaries"`
	Content			string			`json:"content"`   			// can be a hash of the content or the url to the content
	Price			int64			`json:"price"`
}

type Beneficiary struct {
	AccountId		string			`json:"accountId"`
	Percentage		int64			`json:"percentage"`
}

type Account struct {
	Id					string		`json:"id"`
	Name				string		`json:"name"`
	Balance				int64		`json:"balance"`		// optional to keep balance - also bitpesa is possible
	PendingPayments		[]Payment	`json:"pendingPayments"`
}

type Payment struct {
	Recipient			Account		`json:"recipient"`
	Sender				Account		`json:"sender"`
	Amount				int64		`json:"amount"`
	Completed			bool		`json:"completed"`
}

//=================================================================================================================================
//  Evaluation map - Equivalant to an enum for Golang
//  Example:
//  if(!SomeStatus[strings.ToUpper(status)]) { return nil, errors.New("Status not recognized") }
//=================================================================================================================================
var SomeStatus = map[string]bool{
	"somestatus": true,
}

//TODO:
//-- when used with bluemix, add parameter to assign api url for CA

//=================================================================================================================================
//  Index collections - In order to create new IDs dynamically and in progressive sorting
//  Example:
//    signaturesAsBytes, err := stub.GetState(signaturesIndexStr)
//    if err != nil { return nil, errors.New("Failed to get Signatures Index") }
//    fmt.Println("Signature index retrieved")
//
//    // Unmarshal the signatures index
//    var signaturesIndex []string
//    json.Unmarshal(signaturesAsBytes, &signaturesIndex)
//    fmt.Println("Signature index unmarshalled")
//
//    // Create new id for the signature
//    var newSignatureId string
//    newSignatureId = "sg" + strconv.Itoa(len(signaturesIndex) + 1)
//
//    // append the new signature to the index
//    signaturesIndex = append(signaturesIndex, newSignatureId)
//    jsonAsBytes, _ := json.Marshal(signaturesIndex)
//    err = stub.PutState(signaturesIndexStr, jsonAsBytes)
//    if err != nil { return nil, errors.New("Error storing new signaturesIndex into ledger") }
//=================================================================================================================================
var accountIndexStr = "_accounts"
var trackIndexStr = "_tracks"

//==============================================================================================================================
//	Run - Called on chaincode invoke. Takes a function name passed and calls that function. Converts some
//		  initial arguments passed to other things for use in the called function e.g. name -> ecert
//==============================================================================================================================
func (t *SimpleChaincode) Run(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	fmt.Println("run is running " + function)
	return t.Invoke(stub, function, args)
}

func (t *SimpleChaincode) Invoke(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	fmt.Println("invoke is running " + function)

	if function == "init" {
		return t.Init(stub, "init", args)
	} else if function == "add_account" {
		return t.add_track(stub, args)
	} else if function == "add_track" {
		return t.add_track(stub, args)
	} else if function == "register_track" {
		return t.register_track(stub, args)
	}

	return nil, errors.New("Received unknown invoke function name")
}

//=================================================================================================================================
//	Query - Called on chaincode query. Takes a function name passed and calls that function. Passes the
//  		initial arguments passed are passed on to the called function.
//
//  args[0] is the function name
//=================================================================================================================================
func (t *SimpleChaincode) Query(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {

	if args[0] == "get_account" {
		return t.get_account(stub, args[1])
	} else if args[0] == "get_track" {
		return t.get_track(stub, args)
	} else if args[0] == "get_all_tracks" {
		return t.get_all_tracks(stub, args)
	}

	return nil, errors.New("Received unknown query function name")
}

//=================================================================================================================================
//  Main - main - Starts up the chaincode
//=================================================================================================================================

func main() {
	err := shim.Start(new(SimpleChaincode))
	if err != nil {
		fmt.Printf("Error starting SimpleChaincode: %s", err)
	}
}

//==============================================================================================================================
//  Init Function - Called when the user deploys the chaincode
//==============================================================================================================================

func (t *SimpleChaincode) Init(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	return nil, nil
}

//==============================================================================================================================
//  Utility Functions
//==============================================================================================================================

// "create":  true -> create new ID, false -> append the id
func append_id(stub *shim.ChaincodeStub, indexStr string, id string, create bool) ([]byte, error) {

	indexAsBytes, err := stub.GetState(indexStr)
	if err != nil {
		return nil, errors.New("Failed to get " + indexStr)
	}
	fmt.Println(indexStr + " retrieved")

	// Unmarshal the index
	var tmpIndex []string
	json.Unmarshal(indexAsBytes, &tmpIndex)
	fmt.Println(indexStr + " unmarshalled")

	// Create new id
	var newId = id
	if create {
		newId += strconv.Itoa(len(tmpIndex) + 1)
	}

	// append the new id to the index
	tmpIndex = append(tmpIndex, newId)
	jsonAsBytes, _ := json.Marshal(tmpIndex)
	err = stub.PutState(indexStr, jsonAsBytes)
	if err != nil {
		return nil, errors.New("Error storing new " + indexStr + " into ledger")
	}

	return []byte(newId), nil

}

//==============================================================================================================================
//  Certificate Authentication
//==============================================================================================================================

func (t *SimpleChaincode) get_ecert(stub *shim.ChaincodeStub, name string) ([]byte, error) {

	var cert ECertResponse

	response, err := http.Get("http://localhost:5000/registrar/" + name + "/ecert") // Calls out to the HyperLedger REST API to get the ecert of the user with that name

	if err != nil {
		return nil, errors.New("Could not get ecert")
	}

	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body) // Read the response from the http callout into the variable contents

	if err != nil {
		return nil, errors.New("Could not read body")
	}

	err = json.Unmarshal(contents, &cert)

	if err != nil {
		return nil, errors.New("ECert not found for user: " + name)
	}

	return []byte(string(cert.OK)), nil
}

func (t *SimpleChaincode) get_cert_username(stub *shim.ChaincodeStub, encodedCert string) (string, error) {

	decodedCert, err := url.QueryUnescape(encodedCert) // make % etc normal //

	if err != nil {
		return "", errors.New("Could not decode certificate")
	}

	pem, _ := pem.Decode([]byte(decodedCert)) // Make Plain text   //

	x509Cert, err := x509.ParseCertificate(pem.Bytes)

	if err != nil {
		return "", errors.New("Couldn't parse certificate")
	}

	return x509Cert.Subject.CommonName, nil

}

func (t *SimpleChaincode) check_role(stub *shim.ChaincodeStub, encodedCert string) (int64, error) {
	ECertSubjectRole := asn1.ObjectIdentifier{2, 1, 3, 4, 5, 6, 7}

	decodedCert, err := url.QueryUnescape(encodedCert) // make % etc normal //

	if err != nil {
		return -1, errors.New("Could not decode certificate")
	}

	pem, _ := pem.Decode([]byte(decodedCert)) // Make Plain text   //

	x509Cert, err := x509.ParseCertificate(pem.Bytes) // Extract Certificate from argument //

	if err != nil {
		return -1, errors.New("Couldn't parse certificate")
	}

	var role int64
	for _, ext := range x509Cert.Extensions { // Get Role out of Certificate and return it //
		if reflect.DeepEqual(ext.Id, ECertSubjectRole) {
			role, err = strconv.ParseInt(string(ext.Value), 10, len(ext.Value)*8)

			if err != nil {
				return -1, errors.New("Failed parsing role: " + err.Error())
			}
			break
		}
	}

	return role, nil
}

//==============================================================================================================================
//  Invoke Functions
//==============================================================================================================================

func (t *SimpleChaincode) add_account(stub *shim.ChaincodeStub, args []string) ([]byte, error) {

	//Args
	//			0				1
	//		  index		account JSON object (as string)

	id, err := append_id(stub, accountIndexStr, args[0], false)
	if err != nil {
		return nil, errors.New("Error creating new id for user " + args[0])
	}

	err = stub.PutState(string(id), []byte(args[1]))
	if err != nil {
		return nil, errors.New("Error putting user data on ledger")
	}

	return nil, nil
}

func (t *SimpleChaincode) add_track(stub *shim.ChaincodeStub, args []string) ([]byte, error) {

	// args
	// 		0			1
	//	   index	   track JSON object (as string)

	id, err := append_id(stub, trackIndexStr, args[0], false)
	if err != nil {
		return nil, errors.New("Error creating new id for thing " + args[0])
	}

	err = stub.PutState(string(id), []byte(args[1]))
	if err != nil {
		return nil, errors.New("Error putting thing data on ledger")
	}

	return nil, nil

}

// Register that a track is played by an account
// Pay out to the benificiaries of the track
func (t *SimpleChaincode) register_track(stub *shim.ChaincodeStub, args []string) ([]byte, error) {

	// Args
	// 0		1
	// trackId	played_by

	// 1. get track
	trackBytes, err := stub.GetState(args[0])
	if err != nil {
		return nil, errors.New("Could not fetch track " + args[0])
	}
	// 1b. Unmarshal track
	var t Track
	err = json.Unmarshal(trackBytes, &t)
	if err != nil { return nil, errors.New("Could not unmarshal track " + trackBytes ) }

	// 2. get played by account
	playedByBytes, err := stub.GetState(args[1])
	if err != nil { return nil, errors.New("Could not fetch track " + args[1]) }
	// 2b. unmarshal account
	var account_sender Account
	err = json.Unmarshal(playedByBytes, &account_sender)
	if err != nil { return nil, errors.New("Could not unmarshal account " + playedByBytes) }

	// Create array for payments by sender
	var senderPayments []Payment

	// 3. loop through beneficiaries of track
	for _, beneficiary := range t.Beneficiaries {

		// 4. add a PendingPayment to their account

		// 4a. get beneficiary account
		bytes, err := stub.GetState(beneficiary.AccountId)
		if err != nil {
			return nil, errors.New("Unable to get thing with ID: " + track.Id)
		}
		// 4b. unmarshal account
		var account_recipient Account
		json.Unmarshal(bytes, &account_recipient)

		// 4c. calculate amount
		var amount int64
		amount = beneficiary.Percentage * t.Price

		// 4d. create PendingPayment
		var pendingPayment Payment
		pendingPayment.Amount 		= amount
		pendingPayment.Completed 	= false
		pendingPayment.Recipient 	= account_recipient.Id
		pendingPayment.Sender 		= account_sender.Id

		// 4e. append PendingPayment to recipient
		account_recipient.PendingPayments = append(account_recipient.PendingPayments, pendingPayment)

		// 4f. push pendingpayment to senderpayments
		senderPayments = append(senderPayments, pendingPayment)

		// 4g. Put beneficiary back in state
		accReciptientBytes := json.Marshal(account_recipient)
		err = stub.PutState(account_recipient.Id, accReciptientBytes)

	}

	// 5. append senderPayments to sender account
	account_sender.PendingPayments = append(account_sender.PendingPayments, senderPayments)

}

//==============================================================================================================================
//		Query Functions
//==============================================================================================================================

func (t *SimpleChaincode) get_account(stub *shim.ChaincodeStub, userID string) ([]byte, error) {

	bytes, err := stub.GetState(userID)

	if err != nil {
		return nil, errors.New("Could not retrieve information for this user")
	}

	return bytes, nil

}

func (t *SimpleChaincode) get_track(stub *shim.ChaincodeStub, args []string) ([]byte, error) {

	//Args
	//			1
	//		thingID

	bytes, err := stub.GetState(args[1])

	if err != nil {
		return nil, errors.New("Error getting from ledger")
	}

	return bytes, nil

}

func (t *SimpleChaincode) get_all_tracks(stub *shim.ChaincodeStub, args []string) ([]byte, error) {

	indexAsBytes, err := stub.GetState(trackIndexStr)
	if err != nil {
		return nil, errors.New("Failed to get " + trackIndexStr)
	}
	fmt.Println(trackIndexStr + " retrieved")
	s := string(indexAsBytes[:])
	fmt.Println(s)

	// Unmarshal the index
	var trackIndex []string
	errx := json.Unmarshal(indexAsBytes, &trackIndex)
	if errx != nil {
		fmt.Println(errx)
		return nil, errors.New("Failed to get " + trackIndexStr)
	}

	var tracks []Track
	for _, track := range tracks {

		bytes, err := stub.GetState(track.Id)
		if err != nil {
			return nil, errors.New("Unable to get thing with ID: " + track.Id)
		}

		var t Track
		json.Unmarshal(bytes, &t)
		tracks = append(tracks, t)
	}

	tracksAsJsonBytes, _ := json.Marshal(tracks)
	if err != nil {
		return nil, errors.New("Could not convert things to JSON ")
	}

	return tracksAsJsonBytes, nil
}

