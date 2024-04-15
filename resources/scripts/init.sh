#!/bin/sh
awslocal kms create-key --key-spec ECC_SECG_P256K1 --key-usage SIGN_VERIFY
awslocal kms create-key --key-spec ECC_SECG_P256K1 --key-usage SIGN_VERIFY
