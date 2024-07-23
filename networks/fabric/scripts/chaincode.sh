#!/bin/bash

parsePeerConnectionParameters() {
  PEER_CONN_PARMS=()
  PEERS=""
  ORGNAME=""
  PORT=""
  TLS_ROOTCERT_FILE=""
  export CCPORT=""

  while [ "$#" -gt 0 ]; do
    # Check if organization number is a single digit or has multiple digits
    if [ ${#1} -eq 1 ]; then
        PEER="peer1.org0$1" # For single digit, e.g., org1 becomes org01
	ORGNAME="org0$1"
	PORT="600$1"
	CCPORT="700$1"
    else
        PEER="peer1.org$1"  # For multiple digits, e.g., org12 remains org12
	ORGNAME="org$1"
	PORT="60$1"
	CCPORT="70$1"
    fi

    TLS_ROOTCERT_FILE=${PWD}/../certs/chains/peerOrganizations/${ORGNAME}.chains/tlsca/tlsca.${ORGNAME}.chains-cert.pem

    setGlobals $ORGNAME $PORT

    ## Set peer addresses
    if [ -z "$PEERS" ]
    then
	PEERS="$PEER"
    else
	PEERS="$PEERS $PEER"
    fi

    PEER_CONN_PARMS=("${PEER_CONN_PARMS[@]}" --peerAddresses $CORE_PEER_ADDRESS)
    ## Set path to TLS certificate
    TLS_ROOTCERT_FILE=${PWD}/../certs/chains/peerOrganizations/${ORGNAME}.chains/tlsca/tlsca.${ORGNAME}.chains-cert.pem
    TLSINFO=(--tlsRootCertFiles "${TLS_ROOTCERT_FILE}")
    PEER_CONN_PARMS=("${PEER_CONN_PARMS[@]}" "${TLSINFO[@]}")
    # shift by one to get to the next organization
    shift
  done
  echo $PEER_CONN_PARMS
}

function package_chaincode() {
    cd $1
    go get github.com/weids-dev/benchains/chaincodes/wrappers
    cd ../../networks/fabric/scripts/ 
    if [ ! -d "../channel-artifacts" ]; then
	mkdir ../channel-artifacts
    fi
    rm -rf ../channel-artifacts/cc.tar.gz
    peer lifecycle chaincode package ../channel-artifacts/cc.tar.gz --path $1 --lang golang --label wrappers
    export PACKAGE=$(peer lifecycle chaincode calculatepackageid ../channel-artifacts/cc.tar.gz)
    export pkg=${PWD}/../channel-artifacts/cc.tar.gz
    echo $PACKAGE
}

function install_chaincode() {
    setGlobals $1 $2
    peer lifecycle chaincode install $pkg
    peer lifecycle chaincode queryinstalled
}

function approve_chaincode() {
    setGlobals $1 $2
    localpackageID=$3
    peer lifecycle chaincode approveformyorg -o localhost:$4 --ordererTLSHostnameOverride ${ORDERER_NAME} --channelID ${CHANNEL_NAME} --name basic --version 1.0 --package-id $localpackageID --sequence 1 --tls --cafile $ORDERER1_TLS
    peer lifecycle chaincode checkcommitreadiness --channelID ${CHANNEL_NAME} --name basic --version 1.0 --sequence 1 --tls --cafile $ORDERER1_TLS --output json
}

function approve_chaincode_2() {
    # with name of the chaincode
    setGlobals $1 $2
    localpackageID=$3
    ccname=$5
    peer lifecycle chaincode approveformyorg -o localhost:$4 --ordererTLSHostnameOverride ${ORDERER_NAME} --channelID ${CHANNEL_NAME} --name ${ccname} --version 1.0 --package-id $localpackageID --sequence 1 --tls --cafile $ORDERER1_TLS
    peer lifecycle chaincode checkcommitreadiness --channelID ${CHANNEL_NAME} --name ${ccname} --version 1.0 --sequence 1 --tls --cafile $ORDERER1_TLS --output json
}

function commit_chaincode() {
    parsePeerConnectionParameters $@
    peer lifecycle chaincode commit -o localhost:${CCPORT} --ordererTLSHostnameOverride ${ORDERER_NAME} --channelID ${CHANNEL_NAME} --name basic --version 1.0 --sequence 1 --tls --cafile $ORDERER1_TLS "${PEER_CONN_PARMS[@]}" 
    peer lifecycle chaincode querycommitted --channelID ${CHANNEL_NAME} --name basic
}

function commit_chaincode_2() {
    # TODO: fix pasic magic name
    parsePeerConnectionParameters $@
    export CCPORT=7002
    peer lifecycle chaincode commit -o localhost:${CCPORT} --ordererTLSHostnameOverride ${ORDERER_NAME} --channelID ${CHANNEL_NAME} --name pasic --version 1.0 --sequence 1 --tls --cafile $ORDERER1_TLS "${PEER_CONN_PARMS[@]}" 
    peer lifecycle chaincode querycommitted --channelID ${CHANNEL_NAME} --name pasic
}

function query_committed() {
    setGlobals $1 $2
    peer lifecycle chaincode querycommitted --channelID ${CHANNEL_NAME} --name basic
}

# Currency package in the Wrappers
function currency_invoke() {
    parsePeerConnectionParameters $@
    # FABRIC_CFG_PATH=$PWD/../conf/config/

    setGlobals org01 6001
    peer lifecycle chaincode querycommitted --channelID ${CHANNEL_NAME} --name basic

    time peer chaincode invoke -o localhost:7001 --ordererTLSHostnameOverride orderer1.ord01.chains --tls --cafile $ORDERER1_TLS -C chains -n basic "${PEER_CONN_PARMS[@]}" -c '{"function":"InitLedger","Args":[]}'

    sleep 3
    echo "GetAllPlayers"
    time peer chaincode query -C chains -n basic -c '{"Args":["GetAllPlayers"]}'
    time peer chaincode invoke -o localhost:7001 --ordererTLSHostnameOverride orderer1.ord01.chains --tls --cafile "${PWD}/../certs/chains/ordererOrganizations/ord01.chains/tlsca/tlsca.ord01.chains-cert.pem" -C ${CHANNEL_NAME} -n basic "${PEER_CONN_PARMS[@]}" -c '{"function":"CreatePlayer","Args":["AWANG"]}'

    sleep 3
    echo "GetAllPlayers"
    time peer chaincode query -C chains -n basic -c '{"Args":["GetAllPlayers"]}'
    time peer chaincode invoke -o localhost:7001 --ordererTLSHostnameOverride orderer1.ord01.chains --tls --cafile "${PWD}/../certs/chains/ordererOrganizations/ord01.chains/tlsca/tlsca.ord01.chains-cert.pem" -C chains -n basic "${PEER_CONN_PARMS[@]}" -c '{"function":"RecordBankTransaction","Args":["AWANG", "3000", "HSBC9736"]}'
    sleep 3
    time peer chaincode invoke -o localhost:7001 --ordererTLSHostnameOverride orderer1.ord01.chains --tls --cafile "${PWD}/../certs/chains/ordererOrganizations/ord01.chains/tlsca/tlsca.ord01.chains-cert.pem" -C chains -n basic "${PEER_CONN_PARMS[@]}" -c '{"function":"ExchangeInGameCurrency","Args":["AWANG", "HSBC9736", "0.32"]}'
    sleep 3
    time peer chaincode query -C chains -n basic -c '{"Args":["GetAllPlayers"]}'
}

# Currency package in the Wrappers (Test Alternatives)
function currency_invoke_2() {
    parsePeerConnectionParameters $@
    # FABRIC_CFG_PATH=$PWD/../conf/config/

    setGlobals org01 6001
    peer lifecycle chaincode querycommitted --channelID ${CHANNEL_NAME} --name basic

    time peer chaincode invoke -o localhost:7001 --ordererTLSHostnameOverride orderer1.ord01.chains --tls --cafile $ORDERER1_TLS -C ${CHANNEL_NAME} -n basic "${PEER_CONN_PARMS[@]}" -c '{"function":"InitLedger","Args":[]}'

    sleep 3
    echo "GetAllPlayers"
    time peer chaincode query -C ${CHANNEL_NAME} -n basic -c '{"Args":["GetAllPlayers"]}'
    time peer chaincode invoke -o localhost:7001 --ordererTLSHostnameOverride orderer1.ord01.chains --tls --cafile "${PWD}/../certs/chains/ordererOrganizations/ord01.chains/tlsca/tlsca.ord01.chains-cert.pem" -C chains -n basic "${PEER_CONN_PARMS[@]}" -c '{"function":"CreatePlayer","Args":["AWANG"]}'

    sleep 3
    echo "GetAllPlayers"
    time peer chaincode query -C ${CHANNEL_NAME} -n basic -c '{"Args":["GetAllPlayers"]}'
    time peer chaincode invoke -o localhost:7001 --ordererTLSHostnameOverride orderer1.ord01.chains --tls --cafile "${PWD}/../certs/chains/ordererOrganizations/ord01.chains/tlsca/tlsca.ord01.chains-cert.pem" -C ${CHANNEL_NAME} -n basic "${PEER_CONN_PARMS[@]}" -c '{"function":"RecordBankTransaction","Args":["AWANG", "3000", "HSBC9736"]}'
    sleep 3
    time peer chaincode invoke -o localhost:7001 --ordererTLSHostnameOverride orderer1.ord01.chains --tls --cafile "${PWD}/../certs/chains/ordererOrganizations/ord01.chains/tlsca/tlsca.ord01.chains-cert.pem" -C ${CHANNEL_NAME} -n basic "${PEER_CONN_PARMS[@]}" -c '{"function":"ExchangeInGameCurrency","Args":["AWANG", "HSBC9736", "0.32"]}'
    sleep 3
    time peer chaincode query -C ${CHANNEL_NAME} -n basic -c '{"Args":["GetAllPlayers"]}'
}

# Currency package in the Wrappers (Test Alternatives 2) (Only for debug, hardcodded)
function currency_invoke_3() {
    parsePeerConnectionParameters $@
    # FABRIC_CFG_PATH=$PWD/../conf/config/

    export ORDERER1_TLS="${PWD}/../certs/chains/ordererOrganizations/ord02.chains/tlsca/tlsca.ord02.chains-cert.pem"
    export CHANNEL_NAME="chains02"
    setGlobals org02 6002

    peer lifecycle chaincode querycommitted --channelID ${CHANNEL_NAME} --name pasic

    time peer chaincode invoke -o localhost:7002 --ordererTLSHostnameOverride orderer1.ord02.chains --tls --cafile $ORDERER1_TLS -C ${CHANNEL_NAME} -n pasic "${PEER_CONN_PARMS[@]}" -c '{"function":"InitLedger","Args":[]}'

    sleep 3
    echo "GetAllPlayers"
    time peer chaincode query -C ${CHANNEL_NAME} -n pasic -c '{"Args":["GetAllPlayers"]}'
    time peer chaincode invoke -o localhost:7002 --ordererTLSHostnameOverride orderer1.ord02.chains --tls --cafile "${PWD}/../certs/chains/ordererOrganizations/ord02.chains/tlsca/tlsca.ord02.chains-cert.pem" -C ${CHANNEL_NAME} -n pasic "${PEER_CONN_PARMS[@]}" -c '{"function":"CreatePlayer","Args":["AWANG"]}'

    sleep 3
    echo "GetAllPlayers"
    time peer chaincode query -C ${CHANNEL_NAME} -n pasic -c '{"Args":["GetAllPlayers"]}'
    time peer chaincode invoke -o localhost:7002 --ordererTLSHostnameOverride orderer1.ord02.chains --tls --cafile "${PWD}/../certs/chains/ordererOrganizations/ord02.chains/tlsca/tlsca.ord02.chains-cert.pem" -C ${CHANNEL_NAME} -n pasic "${PEER_CONN_PARMS[@]}" -c '{"function":"RecordBankTransaction","Args":["AWANG", "3000", "HSBC9736"]}'
    sleep 3
    time peer chaincode invoke -o localhost:7002 --ordererTLSHostnameOverride orderer1.ord02.chains --tls --cafile "${PWD}/../certs/chains/ordererOrganizations/ord02.chains/tlsca/tlsca.ord02.chains-cert.pem" -C ${CHANNEL_NAME} -n pasic "${PEER_CONN_PARMS[@]}" -c '{"function":"ExchangeInGameCurrency","Args":["AWANG", "HSBC9736", "0.34"]}'
    sleep 3
    time peer chaincode query -C ${CHANNEL_NAME} -n pasic -c '{"Args":["GetAllPlayers"]}'
}

# Currency package in the Wrappers (Test Alternatives)
function currency_query_2() {
    parsePeerConnectionParameters $@
    # FABRIC_CFG_PATH=$PWD/../conf/config/

    setGlobals org01 6001
    peer lifecycle chaincode querycommitted --channelID ${CHANNEL_NAME} --name basic

    echo "GetAllPlayers"
    time peer chaincode query -C ${CHANNEL_NAME} -n basic -c '{"Args":["GetAllPlayers"]}'
}

# Currency package in the Wrappers (Test Alternatives 2) (Only for debug, hardcodded)
function currency_query_3() {
    parsePeerConnectionParameters $@
    # FABRIC_CFG_PATH=$PWD/../conf/config/

    export ORDERER1_TLS="${PWD}/../certs/chains/ordererOrganizations/ord02.chains/tlsca/tlsca.ord02.chains-cert.pem"
    export CHANNEL_NAME="chains02"
    setGlobals org02 6002

    peer lifecycle chaincode querycommitted --channelID ${CHANNEL_NAME} --name pasic

    echo "GetAllPlayers"
    time peer chaincode query -C ${CHANNEL_NAME} -n pasic -c '{"Args":["GetAllPlayers"]}'
}

# Sample atcc testing scripts
function atcc_invoke() {
    parsePeerConnectionParameters $@
    FABRIC_CFG_PATH=$PWD/../conf/config/

    setGlobals org01 6001
    peer lifecycle chaincode querycommitted --channelID chains --name basic

    time peer chaincode invoke -o localhost:7001 --ordererTLSHostnameOverride orderer1.ord01.chains --tls --cafile $ORDERER1_TLS -C chains -n basic "${PEER_CONN_PARMS[@]}" -c '{"function":"InitLedger","Args":[]}'
}

function none() {
    sleep 3
    echo "GetAllAssets:"
    time peer chaincode query -C chains -n basic -c '{"Args":["GetAllAssets"]}'

    sleep 3
    echo "ReadAsset asset6:(peer1)"
    time peer chaincode query -C chains -n basic -c '{"Args":["ReadAsset","asset6"]}'
    sleep 2

    echo "TransferAsset asset6 Christopher"

    time peer chaincode invoke -o localhost:7001 --ordererTLSHostnameOverride orderer1.ord01.chains --tls --cafile $ORDERER1_TLS -C chains -n basic "${PEER_CONN_PARMS[@]}" -c '{"function":"TransferAsset","Args":["asset6","Christopher"]}'

    sleep 2

    echo "ReadAsset asset6:(peer2)"
    setGlobals org02 6002
    time peer chaincode query -C chains -n basic -c '{"Args":["ReadAsset","asset6"]}'

    echo "ReadAsset asset6:(peer6)"
    setGlobals org06 6006
    time peer chaincode query -C chains -n basic -c '{"Args":["ReadAsset","asset6"]}'

    atcc_change 1 2 3 4 5 6 7 8
}

function atcc_change() {
    parsePeerConnectionParameters $@
    FABRIC_CFG_PATH=$PWD/../conf/config/
    setGlobals org01 6001
    peer lifecycle chaincode querycommitted --channelID chains --name basic

    time peer chaincode invoke -o localhost:7002 --ordererTLSHostnameOverride orderer1.ord02.chains --tls --cafile "${PWD}/../certs/chains/ordererOrganizations/ord02.chains/tlsca/tlsca.ord02.chains-cert.pem" -C chains -n basic "${PEER_CONN_PARMS[@]}" -c '{"function":"TransferAsset","Args":["asset6","AWANG"]}'

    sleep 3

    echo "ReadAsset asset6:(peer2)"
    setGlobals org02 6002
    time peer chaincode query -C chains -n basic -c '{"Args":["ReadAsset","asset6"]}'
}
