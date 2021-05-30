package messages

import(
	"sync"

	"github.com/pkg/errors"
)

//interface for receiving messages
type Subscriber interface{
	//Next blocks until an message is received through the subscriber
	Next() (Message, error)

	//Close closes the subscriber so it stops receiving new messages
	Close()
}


//interface for sending messages
type Publisher interface{
	//Publish blocks until it is able to send an message through
	Publish(Message) error

	//Closed() check whether the publisher is closed
	Closed() bool
}


//creates a new message subscription
func NewSubscription() (Publisher, Subscriber){
	c := make(chan Message)
	doneC := make(chan struct{})

	pub := &publisher{
		c:			c,
		doneC:	doneC,
	}

	//publisher must keep listening to doneC to see if subscriber has been closed
	go pub.handleClose()

	sub := &subscriber{
		c:			c,
		doneC:	doneC,
	}

	return pub, sub
}


type subscriber struct{
	c			<-chan Message
	doneC		chan<- struct{}
	closed	bool
}

func (sub *subscriber) Next() (Message, error){
	if sub.closed{
		return nil, errors.New("unable to receive next message: subscriber closed")
	}

	return <-sub.c, nil
}

func (sub *subscriber) Close(){
	//signal we are done, so the owner of the sub.c can stop sending new messages
	sub.doneC <- struct{}{}
	close(sub.doneC)

	sub.closed = true
}


type publisher struct{
	c			chan<- Message
	doneC		<-chan struct{}
	closed	bool

	lock sync.RWMutex
}

func (pub *publisher) Publish(msg Message) error{
	if pub.Closed() {
		return errors.New("unable to publish message: publisher closed")
	}

	select{
		case pub.c <- msg:
		case <-pub.doneC:
			return errors.New("subscriber closed while publishing")
	}

	return nil
}

func (pub *publisher) Closed() bool{
	pub.lock.RLock()
	defer pub.lock.RUnlock()

	return pub.closed
}

func (pub *publisher) handleClose(){
	<-pub.doneC

	pub.lock.Lock()
	defer pub.lock.Unlock()
	pub.closed = true
	close(pub.c)
}
