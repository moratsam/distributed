# Distry

Implementation of some algos, math concepts


## HOWTO

Execute the bootstrap bash script. It will use the distry-bootstrap.privkey to generate its identity. Then you can use the bash commands in the **bash** dir to spin up additional nodes and execute some services. You may have to change the IP in the bash scripts to match the IP of your bootstrap node.

### repo structure overview

##### api
	
Implements the connection between grpc and the code.

##### bash

Utility commands to start a node etc.

##### cmd

The entry point of the program (the main function).

##### k8s

Some yamls for deployment to kubernetes. Not working yet because of double-NAT incompatibility with libp2p peer-discovery.

##### messages

Defines the structs used as messages in various protocols, as well as the logic for (un)marshalling (from)to protobuf structs.

##### node

Base logic of a node. Provides things like bootstrapping, identity creation and connection establishment.

##### omni

Implements the base medium through which nodes exchange messages (libp2p's pubsub).

##### proto

The protobuf definitions of services and messages.

##### proto\_gen

The auto-generated proto code.

##### rbc0

Code for Bracha's reliable broadcast.

## rbc0

bracha's reliable broadcast
DOI: 10.1016/0890-5401(87)90054-x
