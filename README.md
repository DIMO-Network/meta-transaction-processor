# Meta-transaction processor

## Usage

Send a CloudEvent with `data` field
```json
{
    "requestId": "2FowjlIXxjSsbGbtwcDbA1gRdXt",
    "to": "0x662f3314e5bb2ea8a9c18c80c93e064fdadf18b1",
    "data": "0xa9059cbb0000000000000000000000008ab6d69308247c8f9af683436cdcf3532b56cb7b00000000000000000000000000000000000000000000016aaa6682dc63480000"
}
```

On the status topic you'll get messages like
```json
{
    "requestId": "2FowjlIXxjSsbGbtwcDbA1gRdXt",
    "type": "Submitted"
    "transaction": {
        "hash": "0x9273c7b49ffed60592206509e6911ce21be6bf6353a49617a73ff2c01075c4b9"
    }
}
```
Here `type` is one of `Submitted`, `Mined`, `Confirmed`. For confirmed transactions, the `transaction` sub-object will have two additional fields: 
```json
{
    "requestId": "2FowjlIXxjSsbGbtwcDbA1gRdXt",
    "type": "Confirmed"
    "transaction": {
        "hash": "0x9273c7b49ffed60592206509e6911ce21be6bf6353a49617a73ff2c01075c4b9",
        "successful": true,
        "logs": [
            {
                "address": "0x662f3314e5bb2ea8a9c18c80c93e064fdadf18b1",
                "topics": [
                    "0x3d0ce9bfc3ed7d6862dbb28b2dea94561fe714a1b4d019aa8af39730d1ad7c3d",
                    "0x000000000000000000000000f2e391f11cd1609679d03a1ac965b1d0432a7007"
                ],
                "data": "0x00000000000000000000000000000000000000000000000003dc2544280ba2b5"
            }
        ]
    }
}
```

## Configuration

The [default settings file](settings.sample.yaml) has reasonable defaults for local development. It assumes you are using the [Hardhat node](https://hardhat.org/hardhat-runner/docs/getting-started#connecting-a-wallet-or-dapp-to-hardhat-network) and has `PRIVATE_KEY_MODE` set to true, which should never be done in production.
