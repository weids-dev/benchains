#/bin/bash
# The main file to start, manipulate the layer 1 fabric network for the benchains

. env.sh

. bin.sh
. certs.sh
. conf.sh
. chaincode.sh

# Channels
function join_channels() {
    join_channel org01 6001
    join_channel org02 6002
    join_channel org03 6003
    join_channel org04 6004
    join_channel org05 6005
    join_channel org06 6006
    join_channel org07 6007
    join_channel org08 6008
}

function set_anchor_peers() {
    set_anchor org01 6001
    set_anchor org02 6002
    set_anchor org03 6003
    set_anchor org04 6004
    set_anchor org05 6005
    set_anchor org06 6006
    set_anchor org07 6007
    set_anchor org08 6008
}

# Chaincode
function install() {
    export pkg=$1
    install_chaincode org01 6001
    install_chaincode org02 6002
    install_chaincode org03 6003
    install_chaincode org04 6004
    install_chaincode org05 6005
    install_chaincode org06 6006
    install_chaincode org07 6007
    install_chaincode org08 6008
}

function approve() {
    echo $PACKAGE
    approve_chaincode org01 6001 $PACKAGE 7001
    approve_chaincode org02 6002 $PACKAGE 7001
    approve_chaincode org03 6003 $PACKAGE 7001
    approve_chaincode org04 6004 $PACKAGE 7001
    approve_chaincode org05 6005 $PACKAGE 7001
    approve_chaincode org06 6006 $PACKAGE 7001
    approve_chaincode org07 6007 $PACKAGE 7001
    approve_chaincode org08 6008 $PACKAGE 7001
}

function commit() {
    commit_chaincode_2 1 2 3 4 5 6 7 8

    query_committed org02 6002
    query_committed org04 6004
    query_committed org06 6006
    query_committed org08 6008
}

# vendor
cd ../../../chaincodes/sample-atcc/
go mod tidy && go mod vendor

# back to scripts
cd ../../networks/fabric/scripts/

# Main script logic
while getopts ":hicntlsaefpdw" opt; do
    case $opt in
	i)
	    fabric_bin   # bin.sh
	    ;;
	c)
	    layer1_certs # certs.sh
	    ;;
	n)
	    start_nodes  # conf.sh
	    ;;
	t)
	    stop_nodes   # conf.sh
	    ;;
	l)
	    create_genesis
	    create_channel 1 2 3
	    join_channels
	    set_anchor_peers
	    ;;
	s)
	    # package_chaincode ${PWD}/../../../chaincodes/sample-atcc/
	    package_chaincode ${PWD}/../../../chaincodes/wrappers/
	    install ${PWD}/../channel-artifacts/cc.tar.gz
	    approve
	    commit
	    ;;
	a)
	    # atcc_invoke 1 2 3 4 5 6 7 8
	    # currency_invoke 1 2 3 4 5 6 7 8
	    # currency_invoke_2 1
	    env-args chains02 ord02.chains chains02  # env.sh
	    set -x
	    currency_invoke_3 2
	    ;;
	e)
	    # single_endorsement (ord01, org01)
	    # layer 1 mainchain
	    # Standardized Workflow
	    env-args single-endorsement ord01.chains chains  # env.sh

	    start_nodes # conf.sh
	    create_genesis
	    create_channel 1

	    join_channel org01 6001

	    # Join mainchain peer to plasma chain
	    export ORDERER_NAME=orderer1.ord02.chains
	    export CHANNEL_NAME=chains02
	    export ORDERER1_TLS="${PWD}/../certs/chains/ordererOrganizations/ord02.chains/tlsca/tlsca.ord02.chains-cert.pem"

	    join_channel org01 6001

	    export ORDERER_NAME=orderer1.ord01.chains
	    export CHANNEL_NAME=chains
	    export ORDERER1_TLS="${PWD}/../certs/chains/ordererOrganizations/ord01.chains/tlsca/tlsca.ord01.chains-cert.pem"

	    package_chaincode ${PWD}/../../../chaincodes/wrappers/
	    export pkg=${PWD}/../channel-artifacts/cc.tar.gz
	    install_chaincode org01 6001
	    echo $PACKAGE

	    approve_chaincode org01 6001 $PACKAGE 7001
	    commit_chaincode 1
	    currency_invoke_2 1
	    ;;
	d)
	    # plasma chain (chains02)
	    env-args chains02 ord02.chains chains02  # env.sh

	    start_nodes # conf.sh
	    create_genesis
	    create_channel 2
	    
	    join_channel org02 6002

	    # package_chaincode ${PWD}/../../../chaincodes/wrappers/
	    # export pkg=${PWD}/../channel-artifacts/cc.tar.gz
	    # install_chaincode org02 6002
	    # echo $PACKAGE
	    # approve_chaincode_2 org02 6002 $PACKAGE 7002 pasic
	    # commit_chaincode 2
	    # query_committed org02 6002
	    # currency_invoke_3 2
	    ;;
	w)
	    # install chaincode for mainchain and plasma chain
	    # run after bringing up all nodes
	    
	    # 1. package chaincode
	    package_chaincode ${PWD}/../../../chaincodes/wrappers/

	    # 2. install chaincode on plasma chains
	    env-args single-endorsement ord02.chains chains02
	    install_chaincode org01 6001 # chains02
	    env-args chains02 ord02.chains chains02
	    install_chaincode org02 6002 # chains02

	    # 3. approve chaincode for plasma chain orderer (chains02) with name "pasic"
	    env-args single-endorsement ord02.chains chains02
	    approve_chaincode_2 org01 6001 $PACKAGE 7002 pasic
	    env-args chains02 ord02.chains chains02
	    approve_chaincode_2 org02 6002 $PACKAGE 7002 pasic

	    # 4. commit chaincode
	    commit_chaincode_2 1 2

	    # 5. execute chanicode on chains02
	    currency_invoke_3 1 2

	    # 6. verify the result
	    currency_query_3 2
	    currency_query_3 1

	    env-args single-endorsement ord01.chains chains
	    currency_query_2 1
	    ;;
	f)
	    env-four-endorsement # env.sh
	    start_nodes # conf.sh
	    create_genesis
	    create_channel 1 2
	    join_channel org01 6001
	    join_channel org02 6002
	    join_channel org03 6003
	    join_channel org04 6004

	    set_anchor org01 6001
	    set_anchor org02 6002
	    set_anchor org03 6003
	    set_anchor org04 6004

	    package_chaincode ${PWD}/../../../chaincodes/wrappers/
	    export pkg=${PWD}/../channel-artifacts/cc.tar.gz
	    install_chaincode org01 6001
	    install_chaincode org02 6002
	    install_chaincode org03 6003
	    install_chaincode org04 6004

	    echo $PACKAGE
	    approve_chaincode org01 6001 $PACKAGE 7001
	    approve_chaincode org02 6002 $PACKAGE 7001
	    approve_chaincode org03 6003 $PACKAGE 7001
	    approve_chaincode org04 6004 $PACKAGE 7001

	    commit_chaincode_2 1 2 3 4
	    query_committed org01 6001
	    query_committed org02 6002
	    query_committed org03 6003
	    query_committed org04 6004
	    ;;
	h)
	    echo "Usage: ./fabric.sh [options]"
	    echo "Options:"
	    echo "  -i   Install the fabric binaries."
	    echo "  -c   Create certificates for the network components."
	    echo "  -n   Start the network nodes (peers and orderers)."
	    echo "  -e   Start a single-endorsement slim network."
	    echo "  -t   Stop the network nodes."
	    echo "  -l   Create the network channel and join nodes."
	    echo "  -s   Install and set up the sample chaincode."
	    echo "  -a   Invoke the installed chaincode."
	    echo "  -h   Display this help message."
	    exit 0
	    ;;
        \?)
            echo "Invalid option: -$OPTARG" >&2
	    ;;
    esac
done
