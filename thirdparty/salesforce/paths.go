package salesforce

//Paths
var LoginUrl = "https://login.salesforce.com/services/oauth2/token"
var DescribePath = "/services/data/v29.0/"
var SObjectDescribePath = DescribePath + "sobjects/"
var ContactBasePath = SObjectDescribePath + "Contact/"
var ContactPath = ContactBasePath + "%v/"
var ContactExternalIdPath = ContactBasePath + "CrowdstartId__c/%v"
var ContactsUpdatedPath = ContactBasePath + "updated/?start=%v&end=%v"

var AccountBasePath = SObjectDescribePath + "Account/"
var AccountPath = AccountBasePath + "%v/"
var AccountExternalIdPath = AccountBasePath + "CrowdstartId__c/%v"
var AccountsUpdatedPath = AccountBasePath + "updated/?start=%v&end=%v"
