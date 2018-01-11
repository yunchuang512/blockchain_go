package main

import(
	"fmt"
	"log"
	"strconv"
)

//startNode
func (cli *CLI) startNode(nodeID,minerAddress string){
	fmt.Printf("Starting node %s\n",nodeID)

	if len(minerAddress)>0{
		if ValidateAddress(minerAddress){
			fmt.Println("Mining is on. Address to receive rewards:",minerAddress)			
		}else{
			log.Panic("Wrong miner address!")
		}
	}
	StartServer(nodeID,minerAddress)
}

//createBlockchain create a new blockchain
func (cli *CLI) createBlockchain(address,nodeID string) {
	if !ValidateAddress(address){
		log.Panic("ERROR: Address is not valid")
	}

	bc:=CreateBlockchain(address,nodeID)
	defer bc.db.Close()

	UTXOSet:=UTXOSet{bc}
	UTXOSet.Reindex()

	fmt.Println("Done!")
}

//send send amount from FROM to TO
func (cli *CLI) send(from,to string,amount int,nodeID string,mineNow bool){
	if !ValidateAddress(from){
		log.Panic("ERROR:Sender address is not valid")
	}
	if !ValidateAddress(to){
		log.Panic("ERROR:Recipient address is not valid")
	}

	bc:=NewBlockchain(nodeID)
	UTXOSet:=UTXOSet{bc}
	defer bc.db.Close()

	wallets,err:=NewWallets(nodeID)
	if err!=nil{
		log.Panic(err)
	}

	wallet:=wallets.GetWallet(from)

	tx:=NewUTXOTransaction(&wallet,to,amount,&UTXOSet)
	if mineNow{
		cbtx:=NewCoinbaseTX(from,"")
		txs:=[]*Transaction{cbtx,tx}

		newBlock:=bc.MineBlock(txs)
		UTXOSet.Update(newBlock)
	}else{
		sendTx(knownNodes[0],tx)
	}
	

	fmt.Println("Send Success!")
}

//getBalance get balance of address
func (cli *CLI) getBalance(address string,nodeID string){
	if !ValidateAddress(address){
		log.Panic("ERROR: Address is not valid")
	}
	bc:=NewBlockchain(nodeID)
	UTXOSet:=UTXOSet{bc}
	defer bc.db.Close()

	balance:=0
	pubKeyHash:=Base58Decode([]byte(address))
	pubKeyHash=pubKeyHash[1:len(pubKeyHash)-4]
	
	UTXOs:=UTXOSet.FindUTXO(pubKeyHash)

	for _,out:=range UTXOs{
		balance+=out.Value
	}
	fmt.Printf("Balance of '%s':%d\n",address,balance)
}

//createWallet create a new wallet
func (cli *CLI) createWallet(nodeID string){
	wallets,_:=NewWallets(nodeID)
	//wallet:=NewWallet()
	address:=wallets.CreateWallet()
	wallets.SaveToFile(nodeID)
	fmt.Println("Your new address:",address)
}

//listAddresses list all addresses
func (cli *CLI) listAddresses(nodeID string){
	wallets,err:=NewWallets(nodeID)
	if err!=nil{
		log.Panic(err)
	}

	addresses:=wallets.GetAddresses()

	for _,address:=range addresses{
		fmt.Println(address)
	}
}

//printChain print the blockchain
func (cli *CLI) printChain(){
	bc:=NewBlockchain("")
	defer bc.db.Close()

	bci:=bc.Iterator()

	for{
		block:=bci.Next()

		fmt.Printf("========== Block %x ==========\n",block.Hash)
		fmt.Printf("Prev hash:%x\n",block.PrevBlockHash)
		fmt.Printf("Hash:%x\n",block.Hash)
		fmt.Printf("Nonce:%d\n",block.Nonce)
		pow:=NewProofOfWork(block)
		fmt.Printf("PoW:%s\n",strconv.FormatBool(pow.Validate()))
		fmt.Printf("Count of Transactions:%d\n",len(block.Transactions))
		for _,tx:=range block.Transactions{
			fmt.Println(tx)
		}
		fmt.Println()

		if len(block.PrevBlockHash)==0{
			break
		}
	}
}


