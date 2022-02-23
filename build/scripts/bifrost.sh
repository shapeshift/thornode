#!/bin/sh

set -o pipefail

CHAIN_ID="${CHAIN_ID:=thorchain}"
# Binance chain config
BINANCE_HOST="${BINANCE_HOST:=http://binance:26660}"
BINANCE_START_BLOCK_HEIGHT="${BINANCE_START_BLOCK_HEIGHT:=0}"

# Bitcoin core chain config
BTC_HOST="${BTC_HOST:=bitcoin:18443}"
BTC_START_BLOCK_HEIGHT="${BTC_START_BLOCK_HEIGHT:=0}"

# Litecoin core chain config
LTC_HOST="${LTC_HOST:=litecoin:18443}"
LTC_START_BLOCK_HEIGHT="${LTC_START_BLOCK_HEIGHT:=0}"

# Ethereum chain config
ETH_HOST="${ETH_HOST:=http://ethereum:8545}"
ETH_START_BLOCK_HEIGHT="${ETH_START_BLOCK_HEIGHT:=0}"

# Dogecoin chain config
DOGE_HOST="${DOGE_HOST:=dogecoin:18332}"
DOGE_START_BLOCK_HEIGHT="${DOGE_START_BLOCK_HEIGHT:=0}"
DOGE_DISABLED="${DOGE_DISABLED:=false}"

# Terra chain config
TERRA_HOST="${TERRA_HOST:=cosmos-terra-daemon:9090}"
TERRA_START_BLOCK_HEIGHT="${TERRA_START_BLOCK_HEIGHT:=0}"
TERRA_DISABLED="${TERRA_DISABLED:=false}"

# Bitcoin Cash chain config
BCH_HOST="${BCH_HOST:=bitcoin-cash:18443}"
BCH_START_BLOCK_HEIGHT="${BCH_START_BLOCK_HEIGHT:=0}"

DB_PATH="${DB_PATH:=/var/data}"
CHAIN_API="${CHAIN_API:=127.0.0.1:1317}"
CHAIN_RPC="${CHAIN_RPC:=127.0.0.1:26657}"
SIGNER_NAME="${SIGNER_NAME:=thorchain}"
SIGNER_PASSWD="${SIGNER_PASSWD:=password}"
START_BLOCK_HEIGHT="${START_BLOCK_HEIGHT:=0}"
CONTRACT="${CONTRACT:=0x8c2A90D36Ec9F745C9B28B588Cba5e2A978A1656}"

RPC_USER="${RPC_USER:=thorchain}"
RPC_PASSWD="${RPC_PASSWD:=password}"

PPROF_ENABLED="${PPROF_ENABLED:=false}"

THOR_BLOCK_TIME="${THOR_BLOCK_TIME:=5s}"
BLOCK_SCANNER_BACKOFF="${BLOCK_SCANNER_BACKOFF:=5s}"
. "$(dirname "$0")/core.sh"
"$(dirname "$0")/wait-for-thorchain-api.sh" $CHAIN_API

create_thor_user "$SIGNER_NAME" "$SIGNER_PASSWD" "$SIGNER_SEED_PHRASE"

if [ -n "$PEER" ]; then
	OLD_IFS=$IFS
	IFS=","
	SEED_LIST=""
	for SEED in $PEER; do
		# check if we have a hostname we extract the IP
		if ! expr "$SEED" : '[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*$' >/dev/null; then
			SEED=$(host "$SEED" | awk '{print $4}')
		fi
		SEED_ID=$(curl -m 10 -sL --fail "http://$SEED:6040/p2pid") || continue
		SEED="/ip4/$SEED/tcp/5040/ipfs/$SEED_ID"
		if [ -z "$SEED_LIST" ]; then
			SEED_LIST="\"$SEED\""
		else
			SEED_LIST="$SEED_LIST,\"$SEED\""
		fi
	done
	IFS=$OLD_IFS
	PEER=$SEED_LIST
fi

OBSERVER_PATH=$DB_PATH/bifrost/observer/
SIGNER_PATH=$DB_PATH/bifrost/signer/

mkdir -p $SIGNER_PATH $OBSERVER_PATH /etc/bifrost

# Generate bifrost config file
echo "{
    \"thorchain\": {
        \"chain_id\": \"$CHAIN_ID\",
        \"chain_host\": \"$CHAIN_API\",
        \"chain_rpc\": \"$CHAIN_RPC\",
        \"signer_name\": \"$SIGNER_NAME\"
    },
    \"metrics\": {
        \"enabled\": true,
        \"pprof_enabled\": $PPROF_ENABLED
    },
    \"chains\": [
      {
        \"chain_id\": \"BNB\",
        \"rpc_host\": \"$BINANCE_HOST\",
        \"block_scanner\": {
          \"rpc_host\": \"$BINANCE_HOST\",
          \"enforce_block_height\": false,
          \"block_scan_processors\": 1,
          \"block_height_discover_back_off\": \"0.3s\",
          \"block_retry_interval\": \"10s\",
          \"chain_id\": \"BNB\",
          \"http_request_timeout\": \"30s\",
          \"http_request_read_timeout\": \"30s\",
          \"http_request_write_timeout\": \"30s\",
          \"max_http_request_retry\": 10,
          \"start_block_height\": $BINANCE_START_BLOCK_HEIGHT,
          \"db_path\": \"$OBSERVER_PATH\"
        }
      },
      {
        \"chain_id\": \"BTC\",
        \"rpc_host\": \"$BTC_HOST\",
        \"username\": \"$RPC_USER\",
        \"password\": \"$RPC_PASSWD\",
        \"http_post_mode\": 1,
        \"disable_tls\": 1,
        \"block_scanner\": {
          \"rpc_host\": \"$BTC_HOST\",
          \"enforce_block_height\": false,
          \"block_scan_processors\": 1,
          \"block_height_discover_back_off\": \"$BLOCK_SCANNER_BACKOFF\",
          \"block_retry_interval\": \"10s\",
          \"chain_id\": \"BTC\",
          \"http_request_timeout\": \"30s\",
          \"http_request_read_timeout\": \"30s\",
          \"http_request_write_timeout\": \"30s\",
          \"max_http_request_retry\": 10,
          \"start_block_height\": $BTC_START_BLOCK_HEIGHT,
          \"db_path\": \"$OBSERVER_PATH\"
        }
      },
      {
        \"chain_id\": \"DOGE\",
        \"rpc_host\": \"$DOGE_HOST\",
        \"username\": \"$RPC_USER\",
        \"password\": \"$RPC_PASSWD\",
        \"http_post_mode\": 1,
        \"disable_tls\": 1,
        \"disabled\": $DOGE_DISABLED,
        \"block_scanner\": {
          \"rpc_host\": \"$DOGE_HOST\",
          \"enforce_block_height\": false,
          \"block_scan_processors\": 1,
          \"block_height_discover_back_off\": \"$BLOCK_SCANNER_BACKOFF\",
          \"block_retry_interval\": \"10s\",
          \"chain_id\": \"DOGE\",
          \"http_request_timeout\": \"30s\",
          \"http_request_read_timeout\": \"30s\",
          \"http_request_write_timeout\": \"30s\",
          \"max_http_request_retry\": 10,
          \"start_block_height\": $DOGE_START_BLOCK_HEIGHT,
          \"db_path\": \"$OBSERVER_PATH\"
        }
      },
      {
        \"chain_id\": \"TERRA\",
        \"rpc_host\": \"$TERRA_HOST\",
        \"username\": \"$RPC_USER\",
        \"password\": \"$RPC_PASSWD\",
        \"http_post_mode\": 1,
        \"disable_tls\": 1,
        \"disabled\":$TERRA_DISABLED,
        \"block_scanner\": {
          \"rpc_host\": \"$TERRA_HOST\",
          \"enforce_block_height\": false,
          \"block_scan_processors\": 1,
          \"block_height_discover_back_off\": \"$BLOCK_SCANNER_BACKOFF\",
          \"block_retry_interval\": \"10s\",
          \"chain_id\": \"TERRA\",
          \"http_request_timeout\": \"30s\",
          \"http_request_read_timeout\": \"30s\",
          \"http_request_write_timeout\": \"30s\",
          \"max_http_request_retry\": 10,
          \"start_block_height\": $TERRA_START_BLOCK_HEIGHT,
          \"db_path\": \"$OBSERVER_PATH\"
        }
      },
      {
        \"chain_id\": \"LTC\",
        \"rpc_host\": \"$LTC_HOST\",
        \"username\": \"$RPC_USER\",
        \"password\": \"$RPC_PASSWD\",
        \"http_post_mode\": 1,
        \"disable_tls\": 1,
        \"block_scanner\": {
          \"rpc_host\": \"$LTC_HOST\",
          \"enforce_block_height\": false,
          \"block_scan_processors\": 1,
          \"block_height_discover_back_off\": \"$BLOCK_SCANNER_BACKOFF\",
          \"block_retry_interval\": \"10s\",
          \"chain_id\": \"LTC\",
          \"http_request_timeout\": \"30s\",
          \"http_request_read_timeout\": \"30s\",
          \"http_request_write_timeout\": \"30s\",
          \"max_http_request_retry\": 10,
          \"start_block_height\": $LTC_START_BLOCK_HEIGHT,
          \"db_path\": \"$OBSERVER_PATH\"
        }
      },
      {
        \"chain_id\": \"BCH\",
        \"rpc_host\": \"$BCH_HOST\",
        \"username\": \"$RPC_USER\",
        \"password\": \"$RPC_PASSWD\",
        \"http_post_mode\": 1,
        \"disable_tls\": 1,
        \"block_scanner\": {
          \"rpc_host\": \"$BCH_HOST\",
          \"enforce_block_height\": false,
          \"block_scan_processors\": 1,
          \"block_height_discover_back_off\": \"$BLOCK_SCANNER_BACKOFF\",
          \"block_retry_interval\": \"10s\",
          \"chain_id\": \"BCH\",
          \"http_request_timeout\": \"30s\",
          \"http_request_read_timeout\": \"30s\",
          \"http_request_write_timeout\": \"30s\",
          \"max_http_request_retry\": 10,
          \"start_block_height\": $BCH_START_BLOCK_HEIGHT,
          \"db_path\": \"$OBSERVER_PATH\"
        }
      },
      {
        \"chain_id\": \"ETH\",
        \"rpc_host\": \"$ETH_HOST\",
        \"username\": \"$RPC_USER\",
        \"password\": \"$RPC_PASSWD\",
        \"http_post_mode\": 1,
        \"disable_tls\": 1,
        \"contract\": \"$CONTRACT\",
        \"block_scanner\": {
          \"rpc_host\": \"$ETH_HOST\",
          \"enforce_block_height\": false,
          \"block_scan_processors\": 1,
          \"block_height_discover_back_off\": \"$BLOCK_SCANNER_BACKOFF\",
          \"block_retry_interval\": \"10s\",
          \"chain_id\": \"ETH\",
          \"http_request_timeout\": \"30s\",
          \"http_request_read_timeout\": \"30s\",
          \"http_request_write_timeout\": \"30s\",
          \"max_http_request_retry\": 10,
          \"start_block_height\": $ETH_START_BLOCK_HEIGHT,
          \"db_path\": \"$OBSERVER_PATH\"
        }
      }
    ],
    \"tss\": {
        \"bootstrap_peers\": [$PEER],
        \"rendezvous\": \"asgard\",
        \"external_ip\": \"$EXTERNAL_IP\",
        \"p2p_port\": 5040,
        \"info_address\": \":6040\"
    },
    \"signer\": {
      \"signer_db_path\": \"$SIGNER_PATH\",
      \"block_scanner\": {
        \"rpc_host\": \"$CHAIN_RPC\",
        \"start_block_height\": $START_BLOCK_HEIGHT,
        \"enforce_block_height\": false,
        \"block_scan_processors\": 1,
        \"block_height_discover_back_off\": \"$THOR_BLOCK_TIME\",
        \"block_retry_interval\": \"10s\",
        \"start_block_height\": 0,
        \"db_path\": \"$SIGNER_PATH\",
        \"scheme\": \"http\"
      }
    }
}" >/etc/bifrost/config.json
echo "7b225061696c6c696572534b223a7b224e223a32343532323738383739373632383839303737343836303337323738323231363838373837303933323538353339353732353131303336363932313135303332343336383134333636343233333935383336343831343137323034303230303836303933303332383334303536303635313639333439303739313932343837393732313833323431393931393431313330303230393231393331383835333539343536343738323834323634383236313332343934383636313737303137353832373337373931383439393635343839323035373238343232373335373931363134353830343236353139353733333334313535303739393632333432383737303536343630333534393837303336393032303634373835373034393434313035323538373734313633353032373232383332303238313739383833373133313638373531313731363935363336353239353230373239343230333233393137353930303537363532373430303731393139393335303536333533313735343232393036313433303334393533353136343534313331393338303335333037323434323534333135363937363236353133323739303135373338303939303833323934303734393733373934393138303536313837333634353632303537353635393739353130303830363432393538373533313731323734333531373232373737393330363933353536393637323132393834373334323737373836343731383239383539333835383639303032303739393434343734343031353839373830323131323431303430393730343935393438343531333333373132333038343137363039353939332c224c616d6264614e223a31323236313339343339383831343434353338373433303138363339313130383434333933353436363239323639373836323535353138333436303537353136323138343037313833323131363937393138323430373038363032303130303433303436353136343137303238303332353834363734353339353936323433393836303931363230393935393730353635303130343630393635393432363739373238323339313432313332343133303636323437343333303838353038373931333638383935393234393832373434363032383634323131333637383935383037323930323133323539373836363637303737353339393831313731343338353238323330313737343933353138343531303332333932383532343732303532363239333837303831373531333631343135393938333333333231333432333931303636323636303535313630383935373438323830343134373935363938303736393933383936303136303532323131333134303938313434303834373430343530373834333936333332363532363138383134373132323339313738323033303031353334333531373339393735313934393933323031353831333937373332313633383538363432323038353733303339323239363435323233313133323638333036393539353430333435353237323233323133303538393732333732343734323330363137343936353735393333313034383931353637313433373338363938343031323834333732343937393637303138313132303639353332303832373231393433373635373636393136333834313530343032373233363737373231333837342c225068694e223a32343532323738383739373632383839303737343836303337323738323231363838373837303933323538353339353732353131303336363932313135303332343336383134333636343233333935383336343831343137323034303230303836303933303332383334303536303635313639333439303739313932343837393732313833323431393931393431313330303230393231393331383835333539343536343738323834323634383236313332343934383636313737303137353832373337373931383439393635343839323035373238343232373335373931363134353830343236353139353733333334313535303739393632333432383737303536343630333534393837303336393032303634373835373034393434313035323538373734313633353032373232383331393936363636363432363834373832313332353332313130333231373931343936353630383239353931333936313533393837373932303332313034343232363238313936323838313639343830393031353638373932363635333035323337363239343234343738333536343036303033303638373033343739393530333839393836343033313632373935343634333237373137323834343137313436303738343539323930343436323236353336363133393139303830363931303534343436343236313137393434373434393438343631323334393933313531383636323039373833313334323837343737333936383032353638373434393935393334303336323234313339303634313635343433383837353331353333383332373638333030383035343437333535343432373734387d2c224e54696c646569223a32303334323336343032373333323235393235333237333330333138363433363634383532373832373339383132373939373135353036343731363631373839373035383335353939343331393038353438363832383539383737313632323633303830323535323839363738333632343835303738303337383738323037353939383335313237303336323330313739333037313733343937363332313735353135383731363534333638393330353237353038363638363536353736373032353431373730343735313533343330303138313334383235333336353434373430373639323035353134313139393530363330363639383538353737393136303638353036353932343639383338323136353536363932303936313636383234383834383438383432333738373439393034383330393032383031303333323939343831323531363033383332393339353133313037303936383232343330373339373735393235393934323430313738353335333531393638343836353234363437333034393438333234373732373036393039373836333337393536373137303138313131333730363839323733373933363335333631373039323436303330313435383639333735303839343133383135343436333336343038363537363933363833333939373535373832323031373838393238323739393535373532383033363634313233353337313637393635323834333233393133343238303332393634323134363036303633393537373737313535323530303830313633333633383233363039383733343535353839343038343133323231313731373939363835363533372c22483169223a3237323134393238353633313739353634313036343531333039363134343038383830303539333230363532373133363138393735323134313734323836343538343437373432333136373739373336303737313634373031373639363836373136333032353136393338353633353339313739353938393134353139323336393431353238353830373039323330393031313734313636383932303732343835353932343434323134303936343838383932323337393236383633333839323439383439323836353936363730333336393134353537303436303538333238383137323432333630313639363935373134353934353330343030383137303639333632353932373131323230373537343934303034353738373731333439353437363837363934313839383936333537313336393335383439343332313839343135313037323330323230303433383239363438373136373836343937363032393633363631323234323939373639313631323730333439363739333336313134383833343433353936373538393532323536383030323432323037393738333339333234363833353633323330303938383032323834383635313037313434383336343237373634363731333237373034313730313236303535363331363635373334323638343730303233353339353539333630333535353930343537343336383036333539303731383534333539393732383338323431363531353231383538373636303338383739383938313237323234363437353533343735393839323036343536323133343636353433303630333430313837343339393832353030373732322c22483269223a383433393233363237383030363638373738303637333832323833353535363636363733353935333934313531383733393936363532363239363735393535333538303730313335343734393736353031373934373933383834303633383735353833373537333031383633323934343237343236383038313137383837303030303730393931353731353839313733353330373933373339343030383133373830303035313339303339303632323131323638393035323830343334323237323930383634333939303036323539303831393636373939343731313234393535303433303134383333333037323235353639313333313237303935393533363131333338303531313637393438313730363738373139353034363437393333353134333230323737333434383539393037383530323933363030373832343736393535363837323730323639353936363136313539383039393132313332363237363330313736343431363439343230343832383935373439313136393037363536343037313338393231333638383834393632393732343630363135333633333933383933393938333636323938333438363332383730353137313537353732373534303130363439333134353336363639353035373736343530303031303639393634323536363435323338353035303437393234303238303936393938323933313034383439333331353932323937323734373531343231363833303238393530323339333531323436343336393535393239333231383936373131313337383338303936303930303038303136303738343038383732333037313637343733393334312c22416c706861223a313732373737373239363533393831373034393831373934383038373334373236313830383632353037393132353532363936393038323438313436353834383332353536373038393837363435383438323430383334343535313634303131373331323937393133343636373531363735383231383038373537363139313331363638383539333736323035313738363732313237343434383837343832393739333935383932363736313033383836373135373334353735393337303837363835323334313935363134323433383036313333363432383635313334313934393133323734383730313830343532333338383134353134383334333537343632333036323334303938383034333037363635383432393336393037313234373137343939363233323035373830363338333339343334353935313930303539393431363530323736363830353831323638343538363133353132363732363438353338363033323839333434383631313737373333343733343830313635393631323438353439313930383636313037313837333036343839303338313531393037333637333033373836393334333338363538353431303733353731303337333434333739373338313633333834343834363233393430393230333533393535343839343732393036383032373735363135303933343735383436333433313636383637363535373032353731353131333330333730353935333135353936313735303335313338323835393236363534383538313038373333353734303735383937303839373335343432313535303739313632393831303430393136343034313335342c2242657461223a313035303835313535333933353538303834343731313530383930303234333336363036323638363536393639343733393937373836383333333332343430313132383938383030353234343031363431343836393432303734363930343235383432373939383639313132393035373138353038353438373634353730373736323935363431353636363039383130343334373033313231363034383930393139383630333630343836313130343138393832383139363734323631333330353331313537363134313238323233303837333138303831303332373538353839303836373833313933353738383431343333363738393830343139333236393439313833313633333939313234343531343531333432303434393638353530303236363630363234313738353332303532333938333835353333373133333639303932353136303834383837353535353334333534363135323138323036323533313830323737313238393932363132303837353733303932303131353730393232373631333731333336313034303534383132313736383630343231383938393038393030363030303731363531393731383937323735323332323631393136353334333231383030323832353931333834353434353838363839353534353834333435313039363034383338393337323432333136363530323337393433353638393137323535303131323035343432323239353330373831373235313438323937353133323731323532383830333338343735313036323634373139303535363733363136363935343333333436393533333131333835323938333135313936353836362c2250223a36393233333439373539363433373138343336363130343532363230383535343736373538333135323734393438303433343437313332343035313933383135303039383937393736313837393530353838353331363838333833333931363333383738353831323939353731343235353436373531323936323932343631363632303537353431363538333832363733303732303232373130323039393536303738343931393238373535383339363036303031353734373535303135393234383434303332363032353430313432373632383935313839333732343839323533313236363033333833353631313135393035313930353236333036323634303330383137383137373033393733353939373034323937333230353335393332333634383038363733373936333636313835392c2251223a37333435353634313839383632323938323739353433343636343839363038333931353136373332393034393535343932373734323738373335373436353838323030303733333631383936393536343830303333303936333132393538373837323037353035393938373835343438393035303136323537313138303439333837303331363237313233393836363438303835393633373730313234393239373834313635333733373535303738353938343330343238313332373731393232303132383035323639323433363537313833383831343531313833303137393636363032333831363635303637303933353937353632303437343033353831363730353537373336353630373139303533373838363534363734353139333839393532303138353436333932363133343531317d" >/etc/bifrost/preparam.data

export SIGNER_PASSWD
exec "$@"
