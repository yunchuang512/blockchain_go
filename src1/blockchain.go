package main

import(
	"encoding/hex"
	"fmt"
	"log"
	"github.com/boltdb/bolt"
	"os"
)

const dbFile="blockchain2.db"	
const blocksBucket="blocks"
const genesisCoinbaseData="The Times 8/Jan/2018"

type Blockchain struct{
	tip []byte
	db *bolt.DB
}

type BlockchainIterator struct{
	currentHash []byte
	db *bolt.DB
}

func (bc *Blockchain) MineBlock(transactions []*Transaction){
	var lastHash []byte

	err:=bc.db.View(func(tx *bolt.Tx) error{
		b:=tx.Bucket([]byte(blocksBucket))
		lastHash=b.Get([]byte("1"))

		return nil
	})
	if err!=nil{
		log.Panic(err)
	}

	newBlock:=NewBlock(transactions,lastHash)

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

}


func (bc *Blockchain) Iterator() *BlockchainIterator{
	bci:=&BlockchainIterator{bc.tip,bc.db}

	return bci
}

func (i *BlockchainIterator)Next()*Block{
	var block *Block
	err:=i.db.View(func(tx *bolt.Tx)error{
		b:=tx.Bucket([]byte(blocksBucket))
		encodedBlock:=b.Get(i.currentHash)
		block=DeserializeBlock(encodedBlock)

		return nil
	})

	if err!=nil{
		log.Panic(err)
	}

	i.currentHash=block.PrevBlockHash

	return block
}

func dbExists() bool{
	if _,err:=os.Stat(dbFile); os.IsNotExist(err){
		return false
	}

	return true
}

func NewBlockchain(address string) *Blockchain {
	if dbExists()==false{
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

func CreateBlockchain(address string) *Blockchain{
	if dbExists(){
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

func (bc *Blockchain) FindUnspentTransactions(address string) []Transaction{
	var unspentTXs []Transaction
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
				if out.CanBeUnlockedWith(address){
					unspentTXs=append(unspentTXs,*tx)
				}
			}

			if tx.IsCoinBase()==false{
				for _,in:=range tx.Vin{
					if in.CanUnlockOutputWith(address){
						inTxID:=hex.EncodeToString(in.Txid)
						spentTXOs[inTxID]=append(spentTXOs[inTxID],in.Vout)
					}
				}
			} 
		}

		if len(block.PrevBlockHash)==0{
			break
		}
	}

	return unspentTXs
}

func (bc *Blockchain) FindUTXO(address string) []TXOutput{
	var UTXOs []TXOutput
	unspentTransactions:=bc.FindUnspentTransactions(address)

	for _,tx:=range unspentTransactions{
		for _,out:=range tx.Vout{
			if out.CanBeUnlockedWith(address){
				UTXOs=append(UTXOs,out)
			}
		}
	}

	return UTXOs
}

func (bc *Blockchain) FindSpendableOutputs(address string, amount int) (int,map[string][]int){
	unspentOutputs:=make(map[string][]int)
	unspentTXs:=bc.FindUnspentTransactions(address)

	accumulated:=0

	flag:=0
	for _,tx:=range unspentTXs{
		txID:=hex.EncodeToString(tx.ID)

		for outIdx,out:=range tx.Vout{
			if out.CanBeUnlockedWith(address) && accumulated<amount{
				accumulated+=out.Value
				unspentOutputs[txID]=append(unspentOutputs[txID],outIdx)

				if accumulated>=amount{
					flag=1
					break
				}
			}
		}
		if flag==1{
			break
		}
	}

	return accumulated,unspentOutputs
}