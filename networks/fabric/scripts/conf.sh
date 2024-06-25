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

function start_nodes_plasma() {
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

# Create the genesis block in a .block file based on configtx.yaml
function create_genesis_plasma() {
    which configtxgen

    if [ "$?" -ne 0 ]; then
	echo "configtxgen tool not found."
    fi

    if [ ! -d "../channel-artifacts" ]; then
	mkdir ../channel-artifacts
    fi

    # For chains
    configtxgen -profile Raft -outputBlock ../channel-artifacts/chains.block -channelID chains

    # For plaschains
    export FABRIC_CFG_PATH=${PWD}/../slim/plasma-chain/config/plasconfig
    configtxgen -profile Raft -outputBlock ../channel-artifacts/${CHANNEL_NAME}.block -channelID ${CHANNEL_NAME} 

    export FABRIC_CFG_PATH=${PWD}/../slim/plasma-chain/config
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

        # Create the channel and join the orderer to the channel.
        osnadmin channel join --channelID chains --config-block ${PWD}/../channel-artifacts/chains.block -o localhost:${PORT} --ca-file "$ORDERER_CA" --client-cert "$ORDERER_ADMIN_TLS_SIGN_CERT" --client-key "$ORDERER_ADMIN_TLS_PRIVATE_KEY"
        osnadmin channel list --channelID chains -o localhost:${PORT} --ca-file "$ORDERER_CA" --client-cert "$ORDERER_ADMIN_TLS_SIGN_CERT" --client-key "$ORDERER_ADMIN_TLS_PRIVATE_KEY"
    done
}

function create_channel_plasma() {
    which osnadmin
    if [ "$?" -ne 0 ]; then
        fatalln "osnadmin tool not found. Please run with -i to install it"
    fi

    ORDERER="main"
    PORT="8201"
    # Hardcoded, join two orderers into two channels
    export ORDERER_CA="${PWD}/../certs/plasma/ordererOrganizations/${ORDERER}.chains/tlsca/tlsca.${ORDERER}.chains-cert.pem"
    export ORDERER_ADMIN_TLS_SIGN_CERT="${PWD}/../certs/plasma/ordererOrganizations/${ORDERER}.chains/orderers/orderer.${ORDERER}.chains/tls/server.crt"
    export ORDERER_ADMIN_TLS_PRIVATE_KEY="${PWD}/../certs/plasma/ordererOrganizations/${ORDERER}.chains/orderers/orderer.${ORDERER}.chains/tls/server.key"

    # Create the channel and join the orderer to the channel.
    osnadmin channel join --channelID chains --config-block ${PWD}/../channel-artifacts/chains.block -o localhost:${PORT} --ca-file "$ORDERER_CA" --client-cert "$ORDERER_ADMIN_TLS_SIGN_CERT" --client-key "$ORDERER_ADMIN_TLS_PRIVATE_KEY"
    osnadmin channel list --channelID chains -o localhost:${PORT} --ca-file "$ORDERER_CA" --client-cert "$ORDERER_ADMIN_TLS_SIGN_CERT" --client-key "$ORDERER_ADMIN_TLS_PRIVATE_KEY"

    ORDERER="slim"
    PORT="7201"
    # Hardcoded, join two orderers into two channels
    export ORDERER_CA="${PWD}/../certs/plasma/ordererOrganizations/${ORDERER}.plaschains/tlsca/tlsca.${ORDERER}.plaschains-cert.pem"
    export ORDERER_ADMIN_TLS_SIGN_CERT="${PWD}/../certs/plasma/ordererOrganizations/${ORDERER}.plaschains/orderers/orderer.${ORDERER}.plaschains/tls/server.crt"
    export ORDERER_ADMIN_TLS_PRIVATE_KEY="${PWD}/../certs/plasma/ordererOrganizations/${ORDERER}.plaschains/orderers/orderer.${ORDERER}.plaschains/tls/server.key"

    # Create the channel and join the orderer to the channel.
    osnadmin channel join --channelID plaschains --config-block ${PWD}/../channel-artifacts/plaschains.block -o localhost:${PORT} --ca-file "$ORDERER_CA" --client-cert "$ORDERER_ADMIN_TLS_SIGN_CERT" --client-key "$ORDERER_ADMIN_TLS_PRIVATE_KEY"
    osnadmin channel list --channelID plaschains -o localhost:${PORT} --ca-file "$ORDERER_CA" --client-cert "$ORDERER_ADMIN_TLS_SIGN_CERT" --client-key "$ORDERER_ADMIN_TLS_PRIVATE_KEY"
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
    peer channel getinfo -c chains > info2.txt
    res=$?
    { set +x; } 2>/dev/null
		let rc=$res
		COUNTER=$(expr $COUNTER + 1)
	done
    cat log.txt
    verifyResult $res "After $MAX_RETRY attempts, peer${ORG} has failed to join channel"
}


function peer_join() {
    # peer_join channel_name
    local rc=1
    local COUNTER=1
    local DELAY=2
    local MAX_RETRY=3
    ## Sometimes Join takes time, hence retry
	while [ $rc -ne 0 -a $COUNTER -lt $MAX_RETRY ] ; do
    sleep $DELAY
    set -x # enable detailed logging
    # peer channel
    peer channel join -b ../channel-artifacts/$1.block >&log.txt
    res=$?
    { set +x; } 2>/dev/null
		let rc=$res
		COUNTER=$(expr $COUNTER + 1)
	done
    cat log.txt
    verifyResult $res "After $MAX_RETRY attempts, peer${ORG} has failed to join channel"
}

function join_channel_plasma() {
    # 1. Join peer.main.chains to chains
    local orgname="main"
    local port=5001
    export CORE_PEER_TLS_ENABLED=true
    export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/../certs/plasma/peerOrganizations/${orgname}.chains/tlsca/tlsca.${orgname}.chains-cert.pem
    export CORE_PEER_LOCALMSPID=mainchainsMSP
    export CORE_PEER_MSPCONFIGPATH=${PWD}/../certs/plasma/peerOrganizations/${orgname}.chains/users/Admin@${orgname}.chains/msp
    export CORE_PEER_ADDRESS=localhost:${port}
    peer_join chains

    # 2. Join peer.slim.plaschains to plaschains
    local orgname="slim"
    local port=6001
    export CORE_PEER_TLS_ENABLED=true
    export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/../certs/plasma/peerOrganizations/${orgname}.plaschains/tlsca/tlsca.${orgname}.plaschains-cert.pem
    export CORE_PEER_LOCALMSPID=plaschainsMSP
    export CORE_PEER_MSPCONFIGPATH=${PWD}/../certs/plasma/peerOrganizations/${orgname}.plaschains/users/Admin@${orgname}.plaschains/msp
    export CORE_PEER_ADDRESS=localhost:${port}
    export ORDERER_TLSCA_FILE=${PWD}/../certs/plasma/ordererOrganizations/slim.plaschains/orderers/orderer.slim.plaschains/tls/server.crt
    peer_join plaschains
    peer channel fetch newest newest_plasma.block -c plaschains --orderer localhost:7001 --tls --cafile $ORDERER_TLSCA_FILE

    # 3. Join peer.main.chains to plaschains
    local orgname="main"
    local port=5001
    export CORE_PEER_TLS_ENABLED=true
    export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/../certs/plasma/peerOrganizations/${orgname}.chains/tlsca/tlsca.${orgname}.chains-cert.pem
    export CORE_PEER_LOCALMSPID=mainchainsMSP
    export CORE_PEER_MSPCONFIGPATH=${PWD}/../certs/plasma/peerOrganizations/${orgname}.chains/users/Admin@${orgname}.chains/msp
    export CORE_PEER_ADDRESS=localhost:${port}
    local rc=1
    local COUNTER=1
    local DELAY=2
    local MAX_RETRY=3
    ## Sometimes Join takes time, hence retry
	while [ $rc -ne 0 -a $COUNTER -lt $MAX_RETRY ] ; do
    sleep $DELAY
    set -x # enable detailed logging
    # peer channel
    peer channel join -b newest_plasma.block >&log.txt
    res=$?
    { set +x; } 2>/dev/null
		let rc=$res
		COUNTER=$(expr $COUNTER + 1)
	done
    cat log.txt
    verifyResult $res "After $MAX_RETRY attempts, peer${ORG} has failed to join channel"
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
