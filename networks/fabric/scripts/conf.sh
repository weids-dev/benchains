#!/bin/bash

function start_nodes() {
    # warning if artifacts don't exist
    if [ ! -d "../certs/chains/peerOrganizations" ]; then
	fatalln "Please generate the certificates using -c before bring the nodes up."
    fi

    if [ ! -d "../certs/chains/ordererOrganizations" ]; then
	fatalln "Please generate the certificates using -c before bring the nodes up."
    fi

    DOCKER_SOCK="${DOCKER_SOCK}" docker-compose ${COMPOSE_FILES} up -d 2>&1

    docker ps -a
    if [ $? -ne 0 ]; then
	fatalln "Unable to start network"
    fi
}

# Create the genesis block in a .block file based on configtx.yaml
function create_genesis() {
    which configtxgen
    if [ "$?" -ne 0 ]; then
	echo "configtxgen tool not found."
    fi
    if [ ! -d "../channel-artifacts" ]; then
	mkdir ../channel-artifacts
    fi
    configtxgen -profile Raft -outputBlock ../channel-artifacts/${CHANNEL_NAME}.block -channelID ${CHANNEL_NAME} 
}

function create_channel() {
    which osnadmin
    if [ "$?" -ne 0 ]; then
        fatalln "osnadmin tool not found. Please run with -i to install it"
    fi

    for arg in "$@"
    do
        ORDERER="ord0${arg}" # will only have single digits orderers
        PORT=$((9200 + arg))

        export ORDERER_CA="${PWD}/../certs/chains/ordererOrganizations/${ORDERER}.chains/tlsca/tlsca.${ORDERER}.chains-cert.pem"
        export ORDERER_ADMIN_TLS_SIGN_CERT="${PWD}/../certs/chains/ordererOrganizations/${ORDERER}.chains/orderers/orderer1.${ORDERER}.chains/tls/server.crt"
        export ORDERER_ADMIN_TLS_PRIVATE_KEY="${PWD}/../certs/chains/ordererOrganizations/${ORDERER}.chains/orderers/orderer1.${ORDERER}.chains/tls/server.key"

        # Wait for the orderer to be ready (Issue in WSL2)
        echo "Waiting for orderer to start on port ${PORT}..."
        sleep 5

        # Create the channel and join the orderer to the channel.
        osnadmin channel join --channelID ${CHANNEL_NAME} --config-block ${PWD}/../channel-artifacts/${CHANNEL_NAME}.block -o localhost:${PORT} --ca-file "$ORDERER_CA" --client-cert "$ORDERER_ADMIN_TLS_SIGN_CERT" --client-key "$ORDERER_ADMIN_TLS_PRIVATE_KEY"
        osnadmin channel list --channelID ${CHANNEL_NAME} -o localhost:${PORT} --ca-file "$ORDERER_CA" --client-cert "$ORDERER_ADMIN_TLS_SIGN_CERT" --client-key "$ORDERER_ADMIN_TLS_PRIVATE_KEY"
    done
}

function join_channel() {
    setGlobals $1 $2
    local rc=1
    local COUNTER=1
    local DELAY=2
    local MAX_RETRY=3
    ## Sometimes Join takes time, hence retry
	while [ $rc -ne 0 -a $COUNTER -lt $MAX_RETRY ] ; do
    sleep $DELAY
    set -x # enable detailed logging
    # peer channel
    peer channel join -b ../channel-artifacts/${CHANNEL_NAME}.block >&log.txt
    res=$?
    { set +x; } 2>/dev/null
		let rc=$res
		COUNTER=$(expr $COUNTER + 1)
	done
    cat log.txt
    verifyResult $res "After $MAX_RETRY attempts, peer${ORG} has failed to join channel"
}

function fetch_channel() {
    # fetch the newest block
    # fetch_channel networkname ordererport blockname
    # fetch_channel chains02 7001 newest_chains
    local networkname=$1
    local ordererport=$2
    local blockname=$3
 
    # TODO: Fix this ord02
    export ORDERER_TLSCA_FILE=${PWD}/../certs/chains/ordererOrganizations/ord02.chains/orderers/orderer1.ord02.chains/tls/server.crt
    peer channel fetch newest ${blockname}.block -c ${networkname} --orderer localhost:$ordererport --tls --cafile $ORDERER_TLSCA_FILE
}

function set_anchor() {
    docker exec cli ./scripts/anchor.sh $1 $2
}

function stop_nodes() {
    # This step will also delete all the data in the network
    docker stop $(docker ps -q)
    docker rm $(docker ps -a -q)
    # Clear all the volumes
    docker volume rm $(docker volume ls -q)
}
