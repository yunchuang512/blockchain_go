package main

import(
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"log"
	"golang.org/x/crypto/ripemd160"
	"fmt"
)

const walletversion=byte(0x00)
const addressChecksumLen=4

type Wallet struct{
	PrivateKey ecdsa.PrivateKey
	PublicKey []byte
}

func NewWallet() *Wallet{
	private,public:=newKeyPair()
	wallet:=Wallet{private,public}

	return &wallet
}

func (w Wallet) GetAddress() []byte{
	pubKeyHash:=HashPubKey(w.PublicKey)

	versionedPayload:=append([]byte{walletversion},pubKeyHash...)
	checksum:=checksum(versionedPayload)

	fullPayload:=append(versionedPayload,checksum...)
	fmt.Printf("Full=%x\n",fullPayload)
	address:=Base58Encode(fullPayload)

	return address
}

func HashPubKey(pubKey []byte) []byte{
	publicSHA256:=sha256.Sum256(pubKey)

	RIPEMD160Hasher:=ripemd160.New()

	_,err:=RIPEMD160Hasher.Write(publicSHA256[:])
	if err!=nil{
		log.Panic(err)
	}

	publicRIPEMD160:=RIPEMD160Hasher.Sum(nil)

	return publicRIPEMD160
}

func ValidateAddress(address string) bool{
	pubKeyHash:=Base58Decode([]byte(address))
	//fmt.Printf("%x\n",pubKeyHash)
	actualChecksum:=pubKeyHash[len(pubKeyHash)-addressChecksumLen:]
	//fmt.Printf("actual=%x\n",actualChecksum)
	//version:=pubKeyHash[0]
	pubKeyHash=pubKeyHash[1:len(pubKeyHash)-addressChecksumLen]
	//fmt.Printf("pubKeyHash=%x\n",pubKeyHash)
	targetChecksum:=checksum(append([]byte{walletversion},pubKeyHash...))
	//fmt.Printf("target=%x\n",targetChecksum)

	return bytes.Compare(actualChecksum,targetChecksum)==0
}

func checksum(payload []byte) []byte{
	firstSHA:=sha256.Sum256(payload)
	secondSHA:=sha256.Sum256(firstSHA[:])

	return secondSHA[:addressChecksumLen]
}

func newKeyPair() (ecdsa.PrivateKey,[]byte){
	curve:=elliptic.P256()
	private,err:=ecdsa.GenerateKey(curve,rand.Reader)
	if err!=nil{
		log.Panic(err)
	}
	pubKey:=append(private.PublicKey.X.Bytes(),private.PublicKey.Y.Bytes()...)

	return *private,pubKey
}