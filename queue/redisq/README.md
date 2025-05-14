# redisq

Package `redisq` provides a Redis Stream based message queue.

## Max length of stream

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
