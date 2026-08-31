// SPDX-License-Identifier: GPL-2.0-only
/*
 * Experimental WSL compatibility shim for Hacocoon.
 *
 * Microsoft WSL kernels can provide CONFIG_NF_TABLES=y and
 * CONFIG_NETFILTER_FAMILY_BRIDGE=y while leaving CONFIG_NF_TABLES_BRIDGE
 * disabled. Upstream keeps the bridge default chain type inside
 * net/netfilter/nft_chain_filter.c, so there is no standalone upstream
 * module to load later.
 *
 * This module registers only the missing NFPROTO_BRIDGE default filter
 * chain type through nf_tables' exported registration API. Keep this in
 * sync with the target Microsoft WSL kernel source.
 */

#include <linux/if_ether.h>
#include <linux/module.h>
#include <linux/netfilter_bridge.h>

#include <net/netfilter/nf_tables.h>
#include <net/netfilter/nf_tables_ipv4.h>
#include <net/netfilter/nf_tables_ipv6.h>

static unsigned int haco_nft_do_chain_bridge(
	void *priv,
	struct sk_buff *skb,
	const struct nf_hook_state *state)
{
	struct nft_pktinfo pkt;

	nft_set_pktinfo(&pkt, skb, state);

	switch (eth_hdr(skb)->h_proto) {
	case htons(ETH_P_IP):
		nft_set_pktinfo_ipv4_validate(&pkt);
		break;
	case htons(ETH_P_IPV6):
		nft_set_pktinfo_ipv6_validate(&pkt);
		break;
	default:
		nft_set_pktinfo_unspec(&pkt);
		break;
	}

	return nft_do_chain(&pkt, priv);
}

static const struct nft_chain_type haco_nft_chain_filter_bridge = {
	.name = "filter",
	.type = NFT_CHAIN_T_DEFAULT,
	.family = NFPROTO_BRIDGE,
	.owner = THIS_MODULE,
	.hook_mask = (1 << NF_BR_PRE_ROUTING) |
		     (1 << NF_BR_LOCAL_IN) |
		     (1 << NF_BR_FORWARD) |
		     (1 << NF_BR_LOCAL_OUT) |
		     (1 << NF_BR_POST_ROUTING),
	.hooks = {
		[NF_BR_PRE_ROUTING] = haco_nft_do_chain_bridge,
		[NF_BR_LOCAL_IN] = haco_nft_do_chain_bridge,
		[NF_BR_FORWARD] = haco_nft_do_chain_bridge,
		[NF_BR_LOCAL_OUT] = haco_nft_do_chain_bridge,
		[NF_BR_POST_ROUTING] = haco_nft_do_chain_bridge,
	},
};

static int __init haco_nft_bridge_init(void)
{
	nft_register_chain_type(&haco_nft_chain_filter_bridge);
	pr_info("haco_nft_bridge: registered nftables bridge filter chain type\n");
	return 0;
}

static void __exit haco_nft_bridge_exit(void)
{
	nft_unregister_chain_type(&haco_nft_chain_filter_bridge);
	pr_info("haco_nft_bridge: unregistered nftables bridge filter chain type\n");
}

module_init(haco_nft_bridge_init);
module_exit(haco_nft_bridge_exit);

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Hacocoon contributors");
MODULE_DESCRIPTION("Experimental nftables bridge chain compatibility shim for WSL");
