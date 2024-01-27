#!/bin/bash
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
    commit_chaincode 1 2 3 4 5 6 7 8

    query_committed org02 6002
    query_committed org04 6004
    query_committed org06 6006
    query_committed org08 6008
}

# Main script logic
while getopts ":hicntlsa" opt; do
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
	    package_chaincode ${PWD}/../../../chaincodes/sample-atcc/
	    install ${PWD}/../channel-artifacts/cc.tar.gz
	    approve
	    commit
	    ;;
	a)
	    atcc_invoke 1 2 3 4 5 6 7 8
	    ;;
	h)
	    echo "Usage: ./fabric.sh [options]"
	    echo "Options:"
	    echo "  -i   Install the fabric binaries."
	    echo "  -c   Create certificates for the network components."
	    echo "  -n   Start the network nodes (peers and orderers)."
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
