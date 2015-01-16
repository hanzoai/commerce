package salesforce

//Paths
var LoginUrl = "https://login.salesforce.com/services/oauth2/token"
var DescribePath = "/services/data/v29.0/"
var SObjectDescribePath = DescribePath + "sobjects/"
var ContactQueryPath = DescribePath + "query/?q=SELECT+Id+from+Contact+where+Contact.Email+=+"
var ContactBasePath = SObjectDescribePath + "Contact/"
var ContactPath = ContactBasePath + "%v/"
var ContactUpsertUsingEmailPath = ContactBasePath + "CrowdstartId__c/%v"
var ContactsUpdatedPath = ContactBasePath + "updated/?start=%v&end=%v"
