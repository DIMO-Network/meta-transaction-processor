# Meta-transaction processor

Send a CloudEvent with `data` field
```
{
    "requestId": "2FowjlIXxjSsbGbtwcDbA1gRdXt",
    "to": "0x662f3314e5bb2ea8a9c18c80c93e064fdadf18b1",
    "data": "0xa9059cbb0000000000000000000000008ab6d69308247c8f9af683436cdcf3532b56cb7b00000000000000000000000000000000000000000000016aaa6682dc63480000"
}
```

On the status topic you'll get messages like
```
{
    "requestId": "2FowjlIXxjSsbGbtwcDbA1gRdXt",
    "type": "Submitted"
    "transaction": {
        "hash": "0x9273c7b49ffed60592206509e6911ce21be6bf6353a49617a73ff2c01075c4b9"
    }
}
```
Here `type` is one of `Submitted`, `Mined`, `Confirmed`. For confirmed, the `transaction` sub-object will have two additional fields. 
