package main

import(
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
)

const protocol="tcp"
const nodeVersion=1
const commandLength=12

var nodeAddress string
var knownNodes=[]string{"localhost:3000"}
var miningAddress string
var blocksInTransit=[][]byte{}
var mempool=make(map[string]Transaction)

//version send versin msg
type verzion struct{
	Version int
	BestHeight int
	AddrFrom string
}

//getblocks
type getblocks struct{
	AddrFrom string
}

//inv
type inv struct{
	AddrFrom string
	Type string
	Items [][]byte
}

//getdata
type getdata struct{
	AddrFrom string
	Type string
	ID []byte
}

//block struct
type block struct{
	AddrFrom string
	Block []byte
}

//tx struct
type tx struct{
	AddrFrom string
	Transaction []byte
}

//addr
type addr struct{
	AddrList []string
}

//StartServer starts a server
func StartServer(nodeID,minerAddress string){
	nodeAddress=fmt.Sprintf("localhost:%s",nodeID)
	miningAddress=minerAddress

	ln,err:=net.Listen(protocol,nodeAddress)
	if err!=nil{
		log.Panic(err)
	}
	defer ln.Close()

	bc:=NewBlockchain(nodeID)

	if nodeAddress!=knownNodes[0]{
		sendVersion(knownNodes[0],bc)
	}

	for{
		conn,err:=ln.Accept()
		if err!=nil{
			log.Panic(err)
		}
		go handleConnection(conn,bc)
	}
}

//gobEncode
func gobEncode(data interface{}) []byte{
	var buff bytes.Buffer

	enc:=gob.NewEncoder(&buff)
	err:=enc.Encode(data)
	if err!=nil{
		log.Panic(err)
	}

	return buff.Bytes()
}

//nodeIsKnown
func nodeIsKnown(addr string) bool {
	for _,node:=range knownNodes{
		if node==addr{
			return true
		}
	}
	return false
}

//commandToBytes command to bytes
func commandToBytes(command string) []byte {
	var bytes [commandLength]byte

	for i,c:=range command{
		bytes[i]=byte(c)
	}

	return bytes[:]
}

//bytesToCommand bytes to command
func bytesToCommand(bytes []byte) string {
	var command []byte

	for _,b:=range bytes{
		if b!=0x0{
			command=append(command,b)
		}
	}

	return fmt.Sprintf("%s",command)
}

//extractCommand
func extractCommand(request []byte) []byte{
	return request[:commandLength]
}

//requestBlocks
func requestBlocks(){
	for _,node:=range knownNodes{
		sendGetBlocks(node)
	}
}

//sendAddr
func sendAddr(address string){
	nodes:=addr{knownNodes}
	nodes.AddrList=append(nodes.AddrList,nodeAddress)
	payload:=gobEncode(nodes)
	request:=append(commandToBytes("addr"),payload...)

	sendData(address,request)
}

//sendBlock
func sendBlock(addr string,b *Block){
	data:=block{nodeAddress,b.Serialize()}
	payload:=gobEncode(data)
	request:=append(commandToBytes("block"),payload...)

	sendData(addr,request)
}

//sendData
func sendData(addr string,data []byte){
	conn,err:=net.Dial(protocol,addr)
	if err!=nil{
		fmt.Printf("%s is not available\n",addr)
		var updataNodes []string

		for _,node:=range knownNodes{
			if node!=addr{
				updataNodes=append(updataNodes,node)
			}
		}
		knownNodes=updataNodes
		return
	}
	defer conn.Close()

	_,err=io.Copy(conn,bytes.NewReader(data))
	if err!=nil{
		log.Panic(err)
	}
}

//sendInv
func sendInv(address,kind string,items [][]byte){
	inventory:=inv{nodeAddress,kind,items}
	payload:=gobEncode(inventory)
	request:=append(commandToBytes("inv"),payload...)

	sendData(address,request)
}

//sendGetBlocks
func sendGetBlocks(address string){
	payload:=gobEncode(getblocks{nodeAddress})
	request:=append(commandToBytes("getblocks"),payload...)

	sendData(address,request)
}

//sendGetData
func sendGetData(address,kind string,id []byte){
	payload:=gobEncode(getdata{nodeAddress,kind,id})
	request:=append(commandToBytes("getdata"),payload...)

	sendData(address,request)
}

//sendTx
func sendTx(addr string,tnx *Transaction){
	data:=tx{nodeAddress,tnx.Serialize()}
	payload:=gobEncode(data)
	request:=append(commandToBytes("tx"),payload...)

	sendData(addr,request)
}

//sendVersion
func sendVersion(addr string,bc *Blockchain){
	bestHeith:=bc.GetBestHeight()
	payload:=gobEncode(verzion{nodeVersion,bestHeith,nodeAddress})

	request:=append(commandToBytes("version"),payload...)

	sendData(addr,request)
}

//handleAddr
func handleAddr(request []byte){
	var buff bytes.Buffer
	var payload addr

	buff.Write(request[commandLength:])
	dec:=gob.NewDecoder(&buff)
	err:=dec.Decode(&payload)
	if err!=nil{
		log.Panic(err)
	}

	knownNodes=append(knownNodes,payload.AddrList...)
	fmt.Printf("There are %d knowns nodes now!\n",len(knownNodes))
	requestBlocks()
}
//handleConnection handles connection
func handleConnection(conn net.Conn,bc *Blockchain){
	request,err:=ioutil.ReadAll(conn)
	if err!=nil{
		log.Panic(err)
	}
	command:=bytesToCommand(request[:commandLength])
	fmt.Printf("Received: %s command\n",command)

	switch command{
	case "addr":
		handleAddr(request)
	case "block":
		handleBlock(request,bc)
	case "inv":
		handleInv(request,bc)
	case "getblocks":
		handleGetBlocks(request,bc)
	case "getdata":
		handleGetData(request,bc)
	case "tx":
		handleTx(request,bc)
	case "version":
		handleVersion(request,bc)
	default:
		fmt.Println("Unknown command!")
	}

	conn.Close()
}

//handleVersion handles verion command
func handleVersion(request []byte,bc *Blockchain){
	var buff bytes.Buffer
	var payload verzion

	buff.Write(request[commandLength:])
	dec:=gob.NewDecoder(&buff)
	err:=dec.Decode(&payload)
	if err!=nil{
		log.Panic(err)
	}

	myBestHeight:=bc.GetBestHeight()
	foreignerBestHeight:=payload.BestHeight

	if myBestHeight<foreignerBestHeight{
		sendGetBlocks(payload.AddrFrom)
	}else if myBestHeight>foreignerBestHeight{
		sendVersion(payload.AddrFrom,bc)
	}

	if !nodeIsKnown(payload.AddrFrom){
		knownNodes=append(knownNodes,payload.AddrFrom)
	}
}

//handleGetBlocks
func handleGetBlocks(request []byte,bc *Blockchain){
	var buff bytes.Buffer
	var payload getblocks

	buff.Write(request[commandLength:])
	dec:=gob.NewDecoder(&buff)
	err:=dec.Decode(&payload)
	if err!=nil{
		log.Panic(err)
	}
	blocks:=bc.GetBlockHashes()
	sendInv(payload.AddrFrom,"block",blocks)
}

//handleInv
func handleInv(request []byte,bc *Blockchain){
	var buff bytes.Buffer
	var payload inv

	buff.Write(request[commandLength:])
	dec:=gob.NewDecoder(&buff)
	err:=dec.Decode(&payload)
	if err!=nil{
		log.Panic(err)
	}

	fmt.Printf("Recevied inventory with %d %s\n",len(payload.Items),payload.Type)

	if payload.Type=="block"{
		blocksInTransit=payload.Items

		blockHash:=payload.Items[0]
		sendGetData(payload.AddrFrom,"block",blockHash)

		newInTransit:=[][]byte{}

		for _,b:=range blocksInTransit{
			if bytes.Compare(b,blockHash)!=0{
				newInTransit=append(newInTransit,b)
			}
		}
		blocksInTransit=newInTransit
	}

	if payload.Type=="tx"{
		txID:=payload.Items[0]
		if mempool[hex.EncodeToString(txID)].ID==nil{
			sendGetData(payload.AddrFrom,"tx",txID)
		}
	}
}

//handleGetData
func handleGetData(request []byte,bc *Blockchain){
	var buff bytes.Buffer
	var payload getdata

	buff.Write(request[commandLength:])
	dec:=gob.NewDecoder(&buff)
	err:=dec.Decode(&payload)
	if err!=nil{
		log.Panic(err)
	}

	if payload.Type=="block"{
		block,err:=bc.GetBlock([]byte(payload.ID))
		if err!=nil{
			return
		}
		sendBlock(payload.AddrFrom,&block)
	}

	if payload.Type=="tx"{
		txID:=hex.EncodeToString(payload.ID)
		tx:=mempool[txID]

		sendTx(payload.AddrFrom,&tx)
	}
}

//handleBlock
func handleBlock(request []byte,bc *Blockchain){
	var buff bytes.Buffer
	var payload block

	buff.Write(request[commandLength:])
	dec:=gob.NewDecoder(&buff)
	err:=dec.Decode(&payload)
	if err!=nil{
		log.Panic(err)
	}

	blockData:=payload.Block
	block:=DeserializeBlock(blockData)

	fmt.Println("Recevied a new block!")
	bc.AddBlock(block)

	fmt.Printf("Added block %x\n",block.Hash)

	if len(blocksInTransit)>0{
		blockHash:=blocksInTransit[0]
		sendGetData(payload.AddrFrom,"block",blockHash)

		blocksInTransit=blocksInTransit[1:]
	}else{
		UTXOSet:=UTXOSet{bc}
		UTXOSet.Reindex()
	}
}

//handleTx
func handleTx(request []byte,bc *Blockchain){
	var buff bytes.Buffer
	var payload tx

	buff.Write(request[commandLength:])
	dec:=gob.NewDecoder(&buff)
	err:=dec.Decode(&payload)
	if err!=nil{
		log.Panic(err)
	}

	txData:=payload.Transaction
	tx:=DeserializeTransaction(txData)
	mempool[hex.EncodeToString(tx.ID)]=tx

	if nodeAddress==knownNodes[0]{
		for _,node:=range knownNodes{
			if node !=nodeAddress&&node!=payload.AddrFrom{
				sendInv(node,"tx",[][]byte{tx.ID})
			}
		}
	}else{
		if len(mempool)>=2&&len(miningAddress)>0{

			for {
				var txs []*Transaction

				for id:=range mempool{
					tx:=mempool[id]
					if bc.VerifyTransaction(&tx)==true{
						txs=append(txs,&tx)
					}
					delete(mempool,id)
				}

				if len(txs)==0{
					fmt.Println("All transactions are invalid!Waiting for new ones...")
					return
				}

				cbTx:=NewCoinbaseTX(miningAddress,"")
				txs=append(txs,cbTx)

				newBlock:=bc.MineBlock(txs)
				UTXOSet:=UTXOSet{bc}
				UTXOSet.Reindex()

				/*for _,tx:=range txs{
					txID:=hex.EncodeToString(tx.ID)
					delete(mempool,txID)
				}*/

				for _,node:=range knownNodes{
					if node!=nodeAddress{
						sendInv(node,"block",[][]byte{newBlock.Hash})
					}
				}

				if len(mempool)==0{
					break
				}
			}
		}
	}
}