# rate-limiting

# Algorithm

The token bucket algorithm can be conceptually understood as follows:

* A token is added to the bucket every 1/r seconds.
* The bucket can hold at the most b tokens. If a token arrives when the bucket is full, it is discarded.
* When a packet (network layer PDU) of n bytes arrives, n tokens are removed from the bucket, and the packet is sent to the network.
* If fewer than n tokens are available, no tokens are removed from the bucket, and the packet is considered to be non-conformant.
