package salesforce

//Paths
var LoginUrl = "https://login.salesforce.com/services/oauth2/token"
var DescribePath = "/services/data/v29.0/"
var SObjectDescribePath = DescribePath + "sobjects/"
var ContactQueryPath = DescribePath + "query/?q=SELECT+Id+from+Contact+where+Contact.Email+=+"
var ContactPath = SObjectDescribePath + "Contact/%v"
var ContactUpsertUsingEmailPath = ContactPath + "CrowdstartId__c/%v"
var ContactsUpdatedPath = ContactPath + "updated/?start=%v&end=%v"
