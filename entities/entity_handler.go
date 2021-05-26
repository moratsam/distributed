package entities

import(
	"sync"

	"github.com/pkg/errors"
)

//interface for receiving entities
type Subscriber interface{
	//Next blocks until an entity is received through the subscriber
	Next() (Entity, error)

	//Close closes the subscriber so it stops receiving new entities
	Close()
}


//interface for sending entities
type Publisher interface{
	//Publish blocks until it is able to send an entity through
	Publish(Entity) error

	//Closed() check whether the publisher is closed
	Closed() bool
}


//creates a new entity subscription
func NewSubscription() (Publisher, Subscriber){
	c := make(chan Entity)
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
	c			<-chan Entity
	doneC		chan<- struct{}
	closed	bool
}

func (sub *subscriber) Next() (Entity, error){
	if sub.closed{
		return nil, errors.New("unable to receive next entity: subscriber closed")
	}

	return <-sub.c, nil
}

func (sub *subscriber) Close(){
	//signal we are done, so the owner of the sub.c can stop sending new entities
	sub.doneC <- struct{}{}
	close(sub.doneC)

	sub.closed = true
}


type publisher struct{
	c			chan<- Entity
	doneC		<-chan struct{}
	closed	bool

	lock sync.RWMutex
}

func (pub *publisher) Publish(ent Entity) error{
	if pub.Closed() {
		return errors.New("unable to publish entity: publisher closed")
	}

	select{
		case pub.c <- ent:
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
