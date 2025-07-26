package privateapi

import (
    "github.com/walletera/payments-types/privateapi"
    "github.com/walletera/payments-types/publicapi"
)

// Convert transforms a privateapi.Account to a publicapi.Account.
func Convert(account privateapi.Account) publicapi.Account {
    var out publicapi.Account
    out.InstitutionName = publicapi.OptString(account.InstitutionName)
    out.InstitutionId = publicapi.OptString(account.InstitutionId)
    out.Currency = publicapi.Currency(account.Currency)

    details := account.AccountDetails.OneOf
    switch details.Type {
    case privateapi.CvuAccountDetailsAccountAccountDetailsSum:
        if cvu, ok := details.GetCvuAccountDetails(); ok {
            out.AccountDetails = publicapi.AccountAccountDetails{
                OneOf: publicapi.NewCvuAccountDetailsAccountAccountDetailsSum(
                    convertCvuAccountDetails(cvu),
                ),
            }
        }
    case privateapi.DinopayAccountDetailsAccountAccountDetailsSum:
        if dinopay, ok := details.GetDinopayAccountDetails(); ok {
            out.AccountDetails = publicapi.AccountAccountDetails{
                OneOf: publicapi.NewDinopayAccountDetailsAccountAccountDetailsSum(
                    convertDinopayAccountDetails(dinopay),
                ),
            }
        }
    }
    return out
}

func convertCvuAccountDetails(cvu privateapi.CvuAccountDetails) publicapi.CvuAccountDetails {
    pubCvu := publicapi.CvuAccountDetails{
        AccountType: cvu.AccountType,
        Cuit:        publicapi.OptString(cvu.Cuit),
        RoutingInfo: convertCvuRoutingInfo(cvu.RoutingInfo.OneOf),
    }
    return pubCvu
}

func convertCvuRoutingInfo(ri privateapi.CvuAccountDetailsRoutingInfoSum) publicapi.CvuAccountDetailsRoutingInfo {
    var pubRouting publicapi.CvuAccountDetailsRoutingInfo
    switch ri.Type {
    case privateapi.CvuCvuRoutingInfoCvuAccountDetailsRoutingInfoSum:
        pubRouting.OneOf = publicapi.CvuAccountDetailsRoutingInfoSum{
            Type: publicapi.CvuCvuRoutingInfoCvuAccountDetailsRoutingInfoSum,
            CvuCvuRoutingInfo: publicapi.CvuCvuRoutingInfo{
                Cvu: ri.CvuCvuRoutingInfo.Cvu,
            },
        }
    case privateapi.AliasCvuRoutingInfoCvuAccountDetailsRoutingInfoSum:
        pubRouting.OneOf = publicapi.CvuAccountDetailsRoutingInfoSum{
            Type: publicapi.AliasCvuRoutingInfoCvuAccountDetailsRoutingInfoSum,
            AliasCvuRoutingInfo: publicapi.AliasCvuRoutingInfo{
                CvuRoutingInfoType: ri.AliasCvuRoutingInfo.CvuRoutingInfoType,
                Alias:              ri.AliasCvuRoutingInfo.Alias,
            },
        }
    }
    return pubRouting
}

func convertDinopayAccountDetails(dinopay privateapi.DinopayAccountDetails) publicapi.DinopayAccountDetails {
    return publicapi.DinopayAccountDetails{
        AccountType:   dinopay.AccountType,
        AccountHolder: dinopay.AccountHolder,
        AccountNumber: dinopay.AccountNumber,
    }
}