import random

# random.SystemRandom is effectively /dev/urandom with some helper utilities,
# see the random docs for details.
__sysrand = random.SystemRandom()

# generateID returns a randomaly-generated 64-bit ID. It is produced using the
# system's cryptographically-secure RNG (/dev/urandom).
#
# TODO(slimsag): make private once we're not dealing directly with protobuf in
# user-facing code.
def generateID():
    return __sysrand.getrandbits(64)

