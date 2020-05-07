import json
import logging
import requests
import os
import math

# Init logging
logging.basicConfig(
        format="%(asctime)s | %(levelname).4s | %(message)s",
        level=os.environ.get("LOGLEVEL", "INFO"),
        )

elastic_ip = "54.164.46.111"
filename = "node_ip_list.json"
buckets = ['testnet-seed.thorchain.info']

def get_thorchain_url(ip_addr, path):
    return 'http://' + ip_addr + ':1317' + path

def get_tendermint_url(ip_addr, path):
    return 'http://' + ip_addr + ':26657' + path

def get_new_ip_list(ip_addr):
    response = requests.get(get_thorchain_url(ip_addr, "/thorchain/nodeaccounts"))
    peers = [x['ip_address'] for x in response.json() if x['status'] == 'active']
    peers.append(ip_addr)
    peers = list(set(peers)) # uniqify

    # filter nodes that are not "caught up"
    results = []    
    for peer in peers:
        response = requests.get(get_tendermint_url(ip_addr, "/status"))
        if not response.json()['result']['sync_info']['catching_up']:
            results.append(peer)

    return results

def main():
    for bucket in buckets:
        logging.info("Processing bucket: %s", bucket)
        resp = requests.get("http://" + bucket)
        resp.raise_for_status()
        orig_ip_list = resp.json()

        orig_ip_list.insert(0, elastic_ip)
        orig_ip_list = list(set(orig_ip_list)) # avoid duplicates

        logging.info("Original IP list: %s", orig_ip_list)

        votes = []
        for ip_addr in orig_ip_list:
            votes += get_new_ip_list(ip_addr)
        logging.info("Updated IP list: %s", votes)

        ip_list = []
        for ip in votes:
            if ip in ip_list:
                continue
            if votes.count(ip) >= math.ceil(len(orig_ip_list) * 2 / 3):
                ip_list.append(ip)

        ip_list = list(set(ip_list)) # avoid duplicates

        if len(ip_list) == 0:
            logging.error("no ip addresses, exiting...")
            exit(1)

        with open(filename, 'w') as outfile:
            json.dump(ip_list, outfile)

        logging.info("Written to file: %s", filename)
        logging.info("Result: %s", ip_list)

if __name__ == "__main__":
    main()
