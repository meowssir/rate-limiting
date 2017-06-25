# rate-limiting

# Algorithm

The token bucket algorithm can be conceptually understood as follows:

* A token is added to the bucket every 1/r seconds.
* The bucket can hold at the most b tokens. If a token arrives when the bucket is full, it is discarded.
* When a packet (network layer PDU) of n bytes arrives, n tokens are removed from the bucket, and the packet is sent to the network.
* If fewer than n tokens are available, no tokens are removed from the bucket, and the packet is considered to be non-conformant.

# Variations

Implementers of this algorithm on platforms lacking the clock resolution necessary to add a single token to the bucket every 1/r seconds may want to consider an alternative formulation. Given the ability to update the token bucket every S milliseconds, the number of tokens to add every S milliseconds = (r*S)/1000.

The formula for the burst size uses the possible transmission rate in bytes/second as an uppercound constraint which is then used to provide the max token burst rate, such that the bucket size is divided by a theoretical max trasmission rate minus the token rate for which we want to update the bucket.

```
max(T) = b/(M-r) if r < M, otherwize it will be infinity.
```
