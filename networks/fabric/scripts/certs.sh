#!/bin/bash

function layer1_certs() {
    # Generate all the certificase for layer1 fabric cluster using cryptogen
    cd ..
    rm -rf certs
    mkdir -p certs certs/chains

    cryptogen generate --config=conf/layer1.yaml --output="certs/chains"
    cryptogen generate --config=conf/orderers.yaml --output="certs/chains"

    cd scripts
}

function plasma_certs() {
    # Generate all the certificase for plasma & main fabric cluster using cryptogen
    cd ..
    rm -rf certs
    mkdir certs certs/chains
    mkdir certs/plasma

    # crypto-config.yaml contains two channels' config
    cryptogen generate --config=conf/crypto-config.yaml --output="certs/plasma"

    cd scripts
}
