package main

import(
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"log"
	"github.com/boltdb/bolt"
	"os"
	"errors"
)

const dbFile="blockchain_%s.db"	
const blocksBucket="blocks"
const genesisCoinbaseData="The Times 8/Jan/2018"

//Blockchain implements interactions with a DB
type Blockchain struct{
	tip []byte
	db *bolt.DB
}

//MineBlock mines a new block with the provided transaction
func (bc *Blockchain) MineBlock(transactions []*Transaction) *Block{
	var lastHash []byte
	var lastHeight int

	for _,tx:=range transactions{
		if bc.VerifyTransaction(tx)!=true{
			log.Panic("ERROR: Invaild transaction")
		}
	}

	err:=bc.db.View(func(tx *bolt.Tx) error{
		b:=tx.Bucket([]byte(blocksBucket))
		lastHash=b.Get([]byte("1"))

		blockData:=b.Get(lastHash)
		block:=DeserializeBlock(blockData)

		lastHeight=block.Height

		return nil
	})
	if err!=nil{
		log.Panic(err)
	}

	newBlock:=NewBlock(transactions,lastHash,lastHeight+1)

	err=bc.db.Update(func(tx *bolt.Tx)error {
		b:=tx.Bucket([]byte(blocksBucket))
		err:=b.Put(newBlock.Hash,newBlock.Serialize())
		if err!=nil{
			log.Panic(err)
		}

		err=b.Put([]byte("1"),newBlock.Hash)
		if err!=nil{
			log.Panic(err)
		}

		bc.tip=newBlock.Hash
		return nil
	})
	if err!=nil{
		log.Panic(err)
	}
	return newBlock
}

//Just db exist
func dbExists(dbFile string) bool{		
	if _,err:=os.Stat(dbFile); os.IsNotExist(err){
		return false
	}

	return true
}

// NewBlockchain creates a new Blockchain with genesis Block
func NewBlockchain(nodeID string) *Blockchain {
	dbFile:=fmt.Sprintf(dbFile,nodeID)
	if dbExists(dbFile)==false{
		fmt.Println("No existing blockchain found,Create one first")
		os.Exit(1)
	}

	var tip []byte
	db,err:=bolt.Open(dbFile,0600,nil)
	if err!=nil{
		log.Panic(err)
	}

	err=db.Update(func(tx *bolt.Tx) error{
		b:=tx.Bucket([]byte(blocksBucket))
		tip=b.Get([]byte("1"))
		return nil
	})

	if err!=nil{
		log.Panic(err)
	}

	bc:=Blockchain{tip,db}

	return &bc
}

//CreateBlockchain creates a new blockchain DB
func CreateBlockchain(address,nodeID string) *Blockchain{
	dbFile:=fmt.Sprintf(dbFile,nodeID)
	if dbExists(dbFile){
		fmt.Println("Blockchain already exists.")
		os.Exit(1)
	}

	var tip []byte
	db,err:=bolt.Open(dbFile,0600,nil)
	if err!=nil{
		log.Panic(err)
	}

	err=db.Update(func(tx *bolt.Tx) error{
		cbtx:=NewCoinbaseTX(address,genesisCoinbaseData)	
		genesis:=NewGenesisBlock(cbtx)

		b,err:=tx.CreateBucket([]byte(blocksBucket))
		if err!=nil{
			log.Panic(err)
		}

		err=b.Put(genesis.Hash,genesis.Serialize())
		if err!=nil{
			log.Panic(err)
		}

		err=b.Put([]byte("1"),genesis.Hash)
		if err!=nil{
			log.Panic(err)
		}
		tip=genesis.Hash
		
		return nil
	})

	if err!=nil{
		log.Panic(err)
	}
	bc:=Blockchain{tip,db}

	return &bc
}


//FindTransaction finds a transaction by its ID
func (bc *Blockchain) FindTransaction(ID []byte) (Transaction,error) {
	bci:=bc.Iterator()

	for{
		block:=bci.Next()

		for _,tx:=range block.Transactions{
			if bytes.Compare(tx.ID,ID)==0{
				return *tx,nil
			}
		}
		if len(block.PrevBlockHash)==0{
			break
		}
	}

	return Transaction{},errors.New("Transaction is not found")
}


//SignTransaction signs inputs of Transaction
func (bc *Blockchain) SignTransaction(tx *Transaction,privKey ecdsa.PrivateKey){
	prevTXs:=make(map[string]Transaction)

	for _,vin:=range tx.Vin{
		prevTX,err:=bc.FindTransaction(vin.Txid)
		if err!=nil{
			log.Panic(err)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)]=prevTX
	}

	tx.Sign(privKey,prevTXs)
}


//VerifyTransaction verifies transaction input signatures
func (bc *Blockchain) VerifyTransaction(tx *Transaction) bool{
	if tx.IsCoinbase(){
		return true
	}

	prevTXs:=make(map[string]Transaction)

	for _,vin:=range tx.Vin{
		prevTX,err:=bc.FindTransaction(vin.Txid)
		if err!=nil{
			log.Panic(err)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)]=prevTX
	}

	return tx.Verify(prevTXs)
}


//FindUTXO finds all unspent transaction outputs and returns transactions with spent outputs removed
func (bc *Blockchain) FindUTXO() map[string]TXOutputs{
	UTXO:=make(map[string]TXOutputs)
	spentTXOs:=make(map[string][]int)
	bci:=bc.Iterator()

	for{
		block:=bci.Next()

		for _,tx:=range block.Transactions{
			txID:=hex.EncodeToString(tx.ID)
			for outIdx,out:=range tx.Vout{	
				flag:=0
				if spentTXOs[txID]!=nil{
					for _,spentOut:=range spentTXOs[txID]{
						if spentOut==outIdx{
							flag=1
							break
						}
					}
				}
				if flag==1{
					continue
				}
				outs:=UTXO[txID]
				outs.Outputs=append(outs.Outputs,out)
				UTXO[txID]=outs			
			}

			if tx.IsCoinbase()==false{
				for _,in:=range tx.Vin{
					inTxID:=hex.EncodeToString(in.Txid)
					spentTXOs[inTxID]=append(spentTXOs[inTxID],in.Vout)
					
				}
			} 
		}

		if len(block.PrevBlockHash)==0{
			break
		}
	}
	return UTXO
}

//AddBlock saves the block into the blockchain
func (bc *Blockchain) AddBlock(block *Block){
	err:=bc.db.Update(func(tx *bolt.Tx) error {
		b:=tx.Bucket([]byte(blocksBucket))
		blockInDb:=b.Get(block.Hash)

		if blockInDb!=nil{
			return nil
		}

		blockData:=block.Serialize()
		err:=b.Put(block.Hash,blockData)
		if err!=nil{
			log.Panic(err)
		}

		lastHash:=b.Get([]byte("1"))
		lastBlockData:=b.Get(lastHash)
		lastBlock:=DeserializeBlock(lastBlockData)

		if block.Height>lastBlock.Height{
			err=b.Put([]byte("1"),block.Hash)
			if err!=nil{
				log.Panic(err)
			}
			bc.tip=block.Hash
		}
		return nil
	})
	if err!=nil{
		log.Panic(err)
	}
}

//GetBestHeight returns the height of the latest block
func (bc *Blockchain) GetBestHeight() int{
	var lastBlock Block

	err:=bc.db.View(func(tx *bolt.Tx) error {
		b:=tx.Bucket([]byte(blocksBucket))
		lastHash:=b.Get([]byte("1"))
		blockData:=b.Get(lastHash)

		lastBlock=*DeserializeBlock(blockData)
		return nil
	})
	if err!=nil{
		log.Panic(err)
	}

	return lastBlock.Height
}

//GetBlockHashes returns a list of hashes of  all the blocks in the chain
func (bc *Blockchain) GetBlockHashes() [][]byte {
	var blocks [][]byte
	bci:=bc.Iterator()

	for {
		block:=bci.Next()

		blocks=append(blocks,block.Hash)
		if len(block.PrevBlockHash)==0{
			break
		}
	}
	return blocks
}

//GetBlock finds a block by its hash and returns it
func (bc *Blockchain) GetBlock(blockHash []byte) (Block,error){
	var block Block

	err:=bc.db.View(func(tx *bolt.Tx) error {
		b:=tx.Bucket([]byte(blocksBucket))

		blockData:=b.Get(blockHash)

		if blockData==nil{
			return errors.New("Block is not found.")
		}

		block=*DeserializeBlock(blockData)
		return nil
	})
	if err!=nil{
		return block,err
	}
	return block,nil
}