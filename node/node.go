package node

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	_"strconv"
	_"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	circuit "github.com/libp2p/go-libp2p-circuit"
	"github.com/libp2p/go-libp2p-core/crypto"
	discovery2 "github.com/libp2p/go-libp2p-core/discovery"
	libp2phost "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	"github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"distributed/entities"

)

const (
	discoveryNamespace	= "/reconquista"
	privKeyFileName		= "mojkljuc.privkey"
)

type Node interface{
	//INTERNAL
	ID() peer.ID
	Multiaddr() string

	Start(ctx context.Context, port uint16, pkFilePath string) error
	Bootstrap(ctx context.Context, nodeAddrs []multiaddr.Multiaddr) error
	Shutdown() error

	getPrivKey() (string, error)
	sign(string, []byte) ([]byte, error)
	verify(interface{}) bool

	//RPCS
	RBC(message string) (bool, error)
}

type node struct{
	logger *zap.Logger
	host libp2phost.Host
	kadDHT *dht.IpfsDHT
	ps *pubsub.PubSub

	multiaddr string
	multiaddrLock sync.RWMutex

	privKey crypto.PrivKey

	bootstrapOnly bool

	omniManager *OmniManager

	entityPublishers		[]entities.Publisher
	entityPublishersLock	sync.RWMutex
}


//---------------------------<HELPERS>
func (n *node) ID() peer.ID{
	if n.host == nil{
		return ""
	}
	return n.host.ID()
}

func (n *node) Multiaddr() string{
	if n.host == nil{
		return ""
	}

	return n.multiaddr
}


func (n *node) publishEntity(ent entities.Entity){
	n.entityPublishersLock.Lock()
	defer n.entityPublishersLock.Unlock()

	//trimming doesn't work (invalid memory error)
	//so for now I'll just comment out the trimmer code
	//var toTrim []int
	for _, pub := range n.entityPublishers{
		if pub.Closed(){
			continue
		} else if err := pub.Publish(ent); err != nil{
			n.logger.Error("failed publishing node entity", zap.Error(err))
		//	n.logger.Info("removing this publisher")
		//	toTrim = append(toTrim, ix)
		}
	}
}

func (n *node) joinEntities(sub entities.Subscriber){
	for{
		ent, err := sub.Next()
		if err != nil{
			n.logger.Error("failed receiving omni manager entity", zap.Error(err))
		}

		n.publishEntity(ent)
	}
}

func (n *node) getPrivateKey(pkFileName string) (crypto.PrivKey, error) {
	var generate bool
	var err error
	var privKeyBytes []byte

	if pkFileName == ""{
		generate = true
	} else{
		privKeyBytes, err = ioutil.ReadFile(pkFileName)
		if os.IsNotExist(err) {
			n.logger.Info("no identity private key file found.", zap.String("pkFileName", pkFileName))
			generate = true
		} else if err != nil {
			return nil, err
		}
	}

	if generate {
		privKey, err := n.generateNewPrivKey()
		if err != nil {
			return nil, err
		}

		privKeyBytes, err := crypto.MarshalPrivateKey(privKey)
		if err != nil {
			return nil, errors.Wrap(err, "marshalling identity private key")
		}

		f, err := os.Create(privKeyFileName)
		if err != nil {
			return nil, errors.Wrap(err, "creating identity private key file")
		}
		defer f.Close()

		if _, err := f.Write(privKeyBytes); err != nil {
			return nil, errors.Wrap(err, "writing identity private key to file")
		}

		return privKey, nil
	}

	privKey, err := crypto.UnmarshalPrivateKey(privKeyBytes)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling identity private key")
	}

	n.logger.Info("loaded identity private key from file")
	return privKey, nil
}

func (n *node) generateNewPrivKey() (crypto.PrivKey, error) {
	n.logger.Info("generating identity private key")
	privKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, errors.Wrap(err, "generating identity private key")
	}
	n.logger.Info("generated new identity private key")

	return privKey, nil
}

func (n *node) subscribeToEntities() (entities.Subscriber, error){
	pub, sub := entities.NewSubscription()
	n.entityPublishersLock.Lock()
	defer n.entityPublishersLock.Unlock()
	n.entityPublishers = append(n.entityPublishers, pub)

	return sub, nil
}

func (n *node) getPrivKey() (string, error){
	rawBytes, err := n.privKey.Raw()
	if err != nil{
		return "", err
	}
	return base64.StdEncoding.EncodeToString(rawBytes), nil
}
//---------------------------</HELPERS>
//---------------------------<SETUP>

func NewNode(logger *zap.Logger, bootstrapOnly bool) Node{
	if logger == nil{
		logger = zap.NewNop()
	}

	return &node{
		logger:			logger,
		host:				nil,
		bootstrapOnly:	bootstrapOnly,
	}
}

func (n *node) Start(ctx context.Context, port uint16, pkFileName string) error{
	n.logger.Info("starting node", zap.Bool("bootstrapOnly", n.bootstrapOnly))

	nodeAddrStrings := []string{fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port)}

	privKey, err := n.getPrivateKey(pkFileName)
	if err != nil {
		return err
	}

	n.logger.Debug("creating libp2p host")
	host, err := libp2p.New(
		ctx,
		libp2p.ListenAddrStrings(nodeAddrStrings...),
		libp2p.Identity(privKey),
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(circuit.OptHop),
	)
	if err != nil{
		return errors.Wrap(err, "creating libp2p host")
	}
	n.host = host
	n.privKey = privKey

	n.logger.Debug("creating pubsub")
	ps, err := pubsub.NewGossipSub(ctx, n.host, pubsub.WithMessageSignaturePolicy(pubsub.StrictSign))
	if err != nil{
		return errors.Wrap(err, "creating pubsub")
	}
	n.ps = ps

	p2pAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", host.ID().Pretty()))
	if err != nil {
		return errors.Wrap(err, "creating host p2p multiaddr")
	}

	var fullAddrs []string
	for _, addr := range host.Addrs() {
		fullAddrs = append(fullAddrs, addr.Encapsulate(p2pAddr).String())
	}
	n.multiaddr = fullAddrs[0]

	n.logger.Info("started node", zap.Strings("p2pAddresses", fullAddrs))

	return nil
}


func (n *node) Bootstrap(ctx context.Context, nodeAddrs []multiaddr.Multiaddr) error{
	var bootstrappers []peer.AddrInfo
	for _, nodeAddr := range nodeAddrs{
		pi, err := peer.AddrInfoFromP2pAddr(nodeAddr)
		if err != nil{
			return errors.Wrap(err, "parsing bootstrapper node address info from p2p address")
		}

		bootstrappers = append(bootstrappers, *pi)
	}

	n.logger.Debug("creating routing DHT")
	kadDHT, err := dht.New(
		ctx,
		n.host,
		dht.BootstrapPeers(bootstrappers...),
		dht.ProtocolPrefix(discoveryNamespace),
		dht.Mode(dht.ModeAutoServer),
	)
	if err != nil{
		return errors.Wrap(err, "creating routing DHT")
	}
	n.kadDHT = kadDHT

	if err := kadDHT.Bootstrap(ctx); err != nil{
		return errors.Wrap(err, "bootstrapping DHT")
	}

	if n.bootstrapOnly{
		return nil
	}

	n.logger.Debug("setting up OmniManager")
	omniManager, omniManagerEvtSub := NewOmniManager(n.logger, n, n.kadDHT, n.ps)
	n.omniManager = omniManager
	n.omniManager.JoinOmnidisk()
	go n.joinEntities(omniManagerEvtSub)

	if len(nodeAddrs) == 0{
		return nil
	}

	//connect to bootstrap nodes
	for _,pi := range bootstrappers{
		if err := n.host.Connect(ctx, pi); err != nil {
			return errors.Wrap(err, "connecting to bootstrap node")
		}
	}

	rd := discovery.NewRoutingDiscovery(kadDHT)

	n.logger.Info("starting advertising thread")
	discovery.Advertise(ctx, rd, discoveryNamespace)

	//try finding more peers
	go func(){
		peersFound := 0
		for{
			//n.logger.Info("looking for peers...")

			peersChan, err := rd.FindPeers(
				ctx,
				discoveryNamespace,
				discovery2.Limit(100),
			)
			if err != nil{
				n.logger.Error("failed trying to find peers", zap.Error(err))
				continue
			}

			//read all channel messages to avoid blocking the peer query
			for range peersChan{
			}

		/*
			n.logger.Info("done looking for peers",
				zap.Int("peerCount", n.host.Peerstore().Peers().Len()),
			)
		*/

		tmpPeersFound := n.host.Peerstore().Peers().Len()
		if tmpPeersFound != peersFound{
			peersFound = tmpPeersFound
			n.logger.Info("done looking for peers",
				zap.Int("peerCount", n.host.Peerstore().Peers().Len()),
			)
			//fmt.Printf("\n\nmy multiaddr: %v\n\n", n.Multiaddr())
		}

		//	fmt.Printf("\n\nhost addrs: %v\n\n", n.host.Addrs())
		//	fmt.Printf("\n\nmy multiaddr: %v\n\n", n.Multiaddr())
			addrs := n.host.Addrs()
			if len(addrs) > 2{
				n.multiaddrLock.RLock()
				n.multiaddr = addrs[len(addrs) - 1].String() + "/p2p/" + n.ID().Pretty() //last one is public addr
				n.multiaddrLock.RUnlock()

		/*
				if !printOnce{
					n.logger.Info("done looking for peers",
						zap.Int("peerCount", n.host.Peerstore().Peers().Len()),
					)
					fmt.Printf("\n\nmy multiaddr: %v\n\n", n.Multiaddr())
					printOnce = true
				}
		*/
			}

			<-time.After(time.Minute)
		}
	}()

	return nil
}


func (n *node) Shutdown() error{
	return n.host.Close()
}

//---------------------------</SETUP>
//---------------------------<HELPERS>
func (n *node) sign(ownerID string, msg []byte) ([]byte, error){
	//XXX IMPORTING OWNER_ID FROM FILE TO SIGN?
	if ownerID != n.ID().Pretty(){
		n.logger.Warn("cannot sign content of foreign owner")
		return make([]byte, 0), errors.New("ownerID does not match node ID")
	}

	return n.privKey.Sign(msg)

/*
	pubkey1 := n.privKey.GetPublic()
	fmt.Printf("pubkey1 matches pubkey from id:  %v\n\n", n.ID().MatchesPublicKey(pubkey1))
	tru, _ := pubkey1.Verify(msg, signature)
	fmt.Printf("pubkey1 says signature is:  %v\n\n", tru)

	pubkey2, _ := n.ID().ExtractPublicKey()
	fmt.Printf("pubkey2 matches pubkey from id:  %v\n\n", n.ID().MatchesPublicKey(pubkey2))
	tru, _ = pubkey2.Verify(msg, signature)
	fmt.Printf("pubkey2 says signature is:  %v\n\n", tru)

	fmt.Printf("pubkey1 equals pubkey2:  %v\n\n", pubkey1.Equals(pubkey2))

	return signature, nil
*/
}

func (n *node) verify(message interface{}) bool{
	return false;
/*
	switch msg := message.(type){
		case messages.RetrievalRequest:
			ownerID, err := peer.Decode(msg.OwnerID)
			if err != nil{
				n.logger.Debug("cannot verify message with invalid owner ID")
				return false
			}
			pubKey, err := ownerID.ExtractPublicKey()
			if err != nil{
				n.logger.Debug("pubkey extraction from owner ID failed")
				return false
			}

			str := msg.Multiaddr + msg.OwnerID + strconv.FormatInt(msg.Timestamp.Unix(), 10)
			ver, err := pubKey.Verify([]byte(str), []byte(msg.Signature))
			if err != nil{
				n.logger.Debug("error during signature verification")
				return false
			}
			return ver

		case entities.Offer:
			components := strings.Split(msg.Multiaddr, "/")
			ownerID, err := peer.Decode(components[len(components)-1])
			fmt.Printf("\n\nthis is ownerID:   '%v'\n\n", ownerID)
			if err != nil{
				n.logger.Debug("cannot verify message with invalid owner ID")
				return false
			}
			pubKey, err := ownerID.ExtractPublicKey()
			if err != nil{
				n.logger.Debug("pubkey extraction from owner ID failed")
				return false
			}

			str := msg.Multiaddr + strconv.FormatUint(uint64(msg.Capacity), 10) + strconv.FormatInt(msg.Timestamp.Unix(), 10)
			str = "kek"
			fmt.Printf("\n\nthis is offer:  %v\n\n", msg)
			fmt.Printf("\n\nthis is offer str:  %v\n\n", str)
			fmt.Printf("\n\nthis is offer signature:  %v\n\n", msg.Signature)
			ver, err := pubKey.Verify([]byte(str), []byte(msg.Signature))
			if err != nil{
				n.logger.Debug("error during signature verification")
				return false
			}
			n.logger.Debug("okej ampak zdej sm tuki")
			fmt.Printf("picku mater nooooo     %v\n\n", ver)
			return ver

		default:
			n.logger.Debug("cannot verify unknown message type")
			return false
	}
*/
}

//---------------------------</HELPERS>
//---------------------------<RPC>

func (n *node) RBC(message string) (bool, error){
	if n.bootstrapOnly{
		return false, errors.New("can't send message on a bootstrap-only node")
	}

	return false, nil
}





//---------------------------</RPC>

/*
*/


