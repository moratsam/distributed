# Distry

Implementation of some algos, math concepts


## HOWTO

Execute the bootstrap bash script. It will use the distry-bootstrap.privkey to generate its identity. Then you can use the bash commands in the **bash** dir to spin up additional nodes and execute some services. You may have to change the IP in the bash scripts to match the IP of your bootstrap node.

#### repo structure overview

### api
	
Implements the connection between grpc and the code.

### bash

Utility commands to start a node etc.

### cmd

The entry point of the program (the main function).

### k8s

Some yamls for deployment to kubernetes. Not working yet because of double-NAT incompatibility with libp2p peer-discovery.

### messages

Defines the structs used as messages in various protocols, as well as the logic for (un)marshalling (from)to protobuf structs.

### node

Base logic of a node. Provides things like bootstrapping, identity creation and connection establishment.

### omni

Implements the base medium through which nodes exchange messages (libp2p's pubsub).

### proto

The protobuf definitions of services and messages.

### proto\_gen

The auto-generated proto code.

##### reliable broadcast

Code for Bracha's reliable broadcast.

### rbc0

bracha's reliable broadcast
DOI: 10.1016/0890-5401(87)90054-x

We consider the following model of a distributed system. The system consists of n processes that communicate by sending messages through a message system. We assume a reliable message system in which no messages are lost or generated. Each process can directly send messages to any other process, and can identify the sender of every message it receives. Up to t of the processes are faulty and may deviate from the protocol. A protocol is called t-resilient if it satisfies the agreement and validity requirements in the presence of up to t faulty processes.

A protocol is a reliable broadcast protocol (*rbc*) if:
1. If process *p* is correct, then all correct processes agree on the value of its messsage;
2. If *p* is faulty than, either all correct processes agree on the same value or none of them accepts any value from *p*.

The following is a *rbc* with 0 <= t < n/3 byzantine faulty processes.

***Broadcast(v)*** 
- **step 0.** (By process p )
	- Send *(initial,v)* to all the processes
	
- **step 1.** Wait until the receipt of
		one *(initial,v)* message
	 	or (n-t) *(echo,v )* messages
		or (t+1) *(ready,v)* messages
		for some v
	- Send *(echo,v)* to all the processes.

- **step 2.** Wait until the receipt of
		(n-t) *(echo,v)* messages
		or t+1 *(ready,v)* messages
		(including messages received in step 1)
		for some v
	- Send *(ready,v)* to all the processes.

- **step 3.** Wait until the receipt of,
		2t+1 *(ready,v)* messages
		(including messages received in step 1 or step 2) for some v.
	- Accept v.


* Lemma 1: If two correct processes *s* and *t* send *(ready, v)* and *(ready, u)* messages, respectively, then *u*=*v*.

PROOF: Let *q* be the first process that sends *(ready, v)* and *r* the first that sends *(ready, u)*. This means *q* must have received >= (n-t) *(echo, v)* messages and *r* must have received >= (n-t) *(echo, u)* messages. Intersection between two (n-t) subsets must includeat least (n-t)-t >= (t+1) elements which means at least one non-faulty process must have sent bot a *(ready, v)* and a *(ready, u)* message. But correct processes can send only one message of each type during a broadcast, hence a contradiction.


* Lemma 2: If two correct processes *q* and *r* accept the values *v* and *u*, respectively, then *u* = *v*.

PROOF: If *q* accepts *v* it must have received >= (2t+1) *(ready, v)* messages, at least (t+1) of which must have come from correct processes. Analogously for *r* and *u*. Hence, by lemma 1, *u* = *v*.


* Lemma 3: If a correct process *q* accepts a value *v* then every other correct process will eventually accept *v*

PROOF: For *q* to accept *v* it must gave received >= (2t+1) *(ready, v)* messages, of which at least (t+1) must have come from correct processes. Which means every process will eventually receive (t+1) *(ready, v)* messages, which means every correct process will eventually issue a *(ready, v)* message. Which means every correct process will eventually receive at least n-t >= 2t+1 *(ready, v)* messages and will thus accept *v*.


* Lemma 4: If a correct process *p* broadcasts *v* then all correct processes accept *v*.

PROOF: Every correct process *q* receives an *(init, v)* message and sends a *(echo, v)* message. Thus every correct process *q* will receive >= n-t *(echo, v)* messages and will send a *(ready, v)* message. Every correct process will receive >= n-t *(ready, v)* messages and will accept *v*. 

##### erasure codes

GF(2^k) addition or subtraction is xor.
To multiply *a* with *b*, imagine the binary written form as a polynomial of some *x* over {0,1}. Wherever there is a '1' in *a* it means add to the final result that power of *x* multiplied by *b*. Which of course translates to just right shift b by that power. This is done for each '1' in *a*. And how are these partial results then added together? Still thinking of the polynomial representation, it becomes obvious that the simply need to be summed up which is just XOR. Thus, multiplication can be easily implemented with a series of bit shifts and XORs.

The outcome of this operation must by divided by the prime polynomial to ensure that the end result remains in GF(2^k). Now, thinking again in terms of polynomials, division is just subtraction of the divisor at the appropriate powers. And subtraction is also just XOR. The process stops once the the remainder is under 2^k, because for every *e* in GF(2^k): e divided by the generator is e.

example: a=33, b=191, generator=0x11d

		00100001 #a
	 * 10111111 #b
	 ~~~~~~~~~~~~~~~~~~
	 _____10111111 #this is the rightmost '1' in a; the free coefficient in the polynomial so just *b* multiplied by 1
  ^ 10111111_____ #this is the second '1'. Here x is raised to the power of 5 so just shift b 5 times. 
    1011101011111 #normal multiplication is finished, result exceeds 2^8 -1


	
 	 1011101011111 #it needs to be divided by the generator to arrive back in GF(2^8)
  / 100011101____ #this is 0x11d
  ~~~~~~~~~~~~~~~~~~~~~
  	 0011010001111 #still > 2^8 -1, so repeat
  ^   100011101__
	 0001011111011 #repeat
  ^    100011101_
    0000011000001 # = 193 < 2^8 -1, end

	
