#!/bin/bash

function layer1_certs() {
    # Generate all the certificase for layer1 fabric cluster using cryptogen
    cd ..
    sudo rm -rf certs
    mkdir -p certs certs/chains

    cryptogen generate --config=conf/layer1.yaml --output="certs/chains"
    cryptogen generate --config=conf/orderers.yaml --output="certs/chains"

    cd scripts
}

