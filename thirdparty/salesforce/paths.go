package salesforce

//Paths
var LoginUrl = "https://login.salesforce.com/services/oauth2/token"
var DescribePath = "/services/data/v30.0/"
var SObjectDescribePath = DescribePath + "sobjects/"
var ContactBasePath = SObjectDescribePath + "Contact/"
var ContactPath = ContactBasePath + "%v/"
var ContactExternalIdPath = ContactBasePath + "HanzoId__c/%v"
var ContactsUpdatedPath = ContactBasePath + "updated/?start=%v&end=%v"

var AccountBasePath = SObjectDescribePath + "Account/"
var AccountPath = AccountBasePath + "%v/"
var AccountExternalIdPath = AccountBasePath + "HanzoId__c/%v"
var AccountsUpdatedPath = AccountBasePath + "updated/?start=%v&end=%v"

var OrderBasePath = SObjectDescribePath + "Order/"
var OrderPath = OrderBasePath + "%v/"
var OrderExternalIdPath = OrderBasePath + "HanzoId__c/%v"
var OrdersUpdatedPath = OrderBasePath + "updated/?start=%v&end=%v"

var ProductBasePath = SObjectDescribePath + "Product2/"
var ProductPath = ProductBasePath + "%v/"
var ProductExternalIdPath = ProductBasePath + "HanzoId__c/%v"
var ProductsUpdatedPath = ProductBasePath + "updated/?start=%v&end=%v"

var PricebookEntryBasePath = SObjectDescribePath + "PricebookEntry/"
var PricebookEntryPath = PricebookEntryBasePath + "%v/"
var PricebookEntryExternalIdPath = PricebookEntryBasePath + "HanzoId__c/%v"
var PricebookEntrysUpdatedPath = PricebookEntryBasePath + "updated/?start=%v&end=%v"

var OrderProductBasePath = SObjectDescribePath + "OrderItem/"
var OrderProductPath = OrderProductBasePath + "%v/"
var OrderProductExternalIdPath = OrderProductBasePath + "HanzoId__c/%v"
var OrderProductsUpdatedPath = OrderProductBasePath + "updated/?start=%v&end=%v"

// These and only these are case sensitive...
var PlaceOrderOrderBasePath = DescribePath + "commerce/sale/order/"
var PlaceOrderOrderPath = PlaceOrderOrderBasePath + "%v/"
