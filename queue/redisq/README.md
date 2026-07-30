# redisq

Package `redisq` provides a Redis Stream based message queue.

## Max length of stream

`MaxLen` is a retention target, not permission to delete unread messages.
Before trimming, the consumer checks every group and keeps the earliest entry
that is still pending or has not yet been delivered. A slow group can therefore
keep the stream above `MaxLen`; the trim task emits stream length, pending, lag,
safe-boundary, and trimmed-count fields so operators can see why.

Pending entries are scanned continuously, including entries that fail after the
consumer starts. After a pending batch, the consumer gives new entries a read
opportunity so one repeatedly failing message cannot block the rest of the
stream.

The JSON object below is 360 characters long when serialized using `JSON.stringify()`.  
Assume each message is 0.5 KB. For 100 streams, this amounts to approximately 50 KB.  
With 10,000 messages per stream, the total size would be around 500 MB.

```json
{
    "glossary": {
        "title": "example glossary",
        "GlossDiv": {
            "title": "S",
            "GlossList": {
                "GlossEntry": {
                    "ID": "SGML",
                    "SortAs": "SGML",
                    "GlossTerm": "Standard Generalized Markup Language",
                    "Acronym": "SGML",
                    "Abbrev": "ISO 8879:1986",
                    "GlossDef": {
                        "para": "A meta-markup language, used to create markup languages such as DocBook.",
                        "GlossSeeAlso": ["GML", "XML"]
                    },
                    "GlossSee": "markup"
                }
            }
        }
    }
}
```
