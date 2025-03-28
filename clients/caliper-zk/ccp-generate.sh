#!/usr/bin/env bash
function one_line_pem {
    echo "`awk 'NF {sub(/\\n/, ""); printf "%s\\\\\\\n",$0;}' $1`"
}
function yaml_ccp {
    local PP=$(one_line_pem $3)
    sed -e "s/\${ORG}/$1/" \
        -e "s/\${P0PORT}/$2/" \
        -e "s#\${PEERPEM}#$PP#" \
        ccp-template.yaml | sed -e $'s/\\\\n/\\\n          /g'
}
ORG=02
P0PORT=6002
PEERPEM=../../networks/fabric/certs/chains/peerOrganizations/org02.chains/tlsca/tlsca.org02.chains-cert.pem
echo "$(yaml_ccp $ORG $P0PORT $PEERPEM)" > connection-org02.yaml