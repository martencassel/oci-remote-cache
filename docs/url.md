# Terminals:
"/v2/", "blobs", "manifests", "tags", "referrers", "uploads"

# Non-terminals:

<repoKey>    ::= <segment>
<repository> ::= <segment> | <segment> "/" <repository>
<segment>    ::= [a-z0-9._-]+
<digest>     ::= <algo> ":" <hex>
<reference>  ::= <tag> | <digest>

# Top-level

<v2-path> ::= "/v2/" | "/v2/" <repoKey> "/" <repository> "/" <verb> ...

# Verb detection

blobs
manifests
tags
referrers

# Parsing

Step 1 - Check for ping
Step 2 - Strip prefix
Step 3 - Extract repoKey
Step 4 - Greedy repository parse until verb
Step 5 - Dispatch on verb
