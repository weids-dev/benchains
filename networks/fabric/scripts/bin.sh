#!/bin/bash

function fabric_bin() {
    cd ../
    rm -rf bi
    curl -sSLO https://raw.githubusercontent.com/hyperledger/fabric/main/scripts/install-fabric.sh
    chmod +x install-fabric.sh
    ./install-fabric.sh binary docker
    rm -rf builders config install-fabric.sh
    cd scripts/
}

