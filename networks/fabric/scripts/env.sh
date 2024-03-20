#!/bin/bash

function env() {
    export PATH="$PWD/../bin:$PATH"
    export PATH="$PWD:$PATH"

    export FABRIC_CFG_PATH=${PWD}/../conf/config

    # Two compose files
    export COMPOSE_FILES="-f ../conf/compose/compose.yaml -f ../conf/compose/docker/docker-compose.yaml"

    # get docker sock path from environment variable
    export sock="${docker_host:-/var/run/docker.sock}"
    export docker_sock="${sock##unix://}"
    export CHANNEL_NAME=chains
    export ORDERER1_TLS="${PWD}/../certs/chains/ordererOrganizations/ord01.chains/tlsca/tlsca.ord01.chains-cert.pem"
}

function env-single-endorsement() {
    export PATH="$PWD/../bin:$PATH"
    export PATH="$PWD:$PATH"

    export FABRIC_CFG_PATH=${PWD}/../slim/single-endorsement/config/

    # Two compose files
    export COMPOSE_FILES="-f ../slim/single-endorsement/compose/compose.yaml -f ../slim/single-endorsement/compose/docker/docker-compose.yaml"

    # get docker sock path from environment variable
    export sock="${docker_host:-/var/run/docker.sock}"
    export docker_sock="${sock##unix://}"
    export CHANNEL_NAME=chains
    export ORDERER1_TLS="${PWD}/../certs/chains/ordererOrganizations/ord01.chains/tlsca/tlsca.ord01.chains-cert.pem"
}

function env-four-endorsement() {
    export PATH="$PWD/../bin:$PATH"
    export PATH="$PWD:$PATH"

    export FABRIC_CFG_PATH=${PWD}/../slim/four-endorsement/config/

    # Two compose files
    export COMPOSE_FILES="-f ../slim/four-endorsement/compose/compose.yaml -f ../slim/four-endorsement/compose/docker/docker-compose.yaml"

    # get docker sock path from environment variable
    export sock="${docker_host:-/var/run/docker.sock}"
    export docker_sock="${sock##unix://}"
    export CHANNEL_NAME=chains
    export ORDERER1_TLS="${PWD}/../certs/chains/ordererOrganizations/ord01.chains/tlsca/tlsca.ord01.chains-cert.pem"
}


# Helper functions:

# println echos string
function println() {
    echo -e "$1"
}

# errorln echos i red color
function errorln() {
    println "${C_RED}${1}${C_RESET}"
}

# fatalln echos in red color and exits with fail status
function fatalln() {
    errorln "$1"
    exit 1
}

function verifyResult() {
    if [ $1 -ne 0 ]; then
	fatalln "$2"
    fi
}

# Set environment variables for the peer org
function setGlobals() {
    # setGlobals orgname, port
    local orgname=$1
    local port=$2
    export CORE_PEER_TLS_ENABLED=true # enable TLS
    export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/../certs/chains/peerOrganizations/${orgname}.chains/tlsca/tlsca.${orgname}.chains-cert.pem
    export CORE_PEER_LOCALMSPID="${orgname}MSP"
    export CORE_PEER_MSPCONFIGPATH=${PWD}/../certs/chains/peerOrganizations/${orgname}.chains/users/Admin@${orgname}.chains/msp
    export CORE_PEER_ADDRESS=localhost:${port}
}

# Set environment variables for use in the CLI container
setGlobalsCLI() {
    # single organization setup
    setGlobals $1 $2
}

env
