var Util = (function() {
  var currencySigns = {
	'usd': '$',
	'aud': '$',
	'cad': '$',
	'eur': '€',
	'gbp': '£',
	'': '$'
  };

  var currencySeparator = '.';
  var currentCurrencyCode = 'usd';
  var currentCurrencySign = currencySigns[currentCurrencyCode];

  return {
    setCurrency: function(code){
      currentCurrencyCode = code;
      currentCurrencySign = currencySigns[code];
    },
    renderUICurrencyFromJSON: function(jsonCurrency) {
      // jsonCurrency is cents
      jsonCurrency = '' + jsonCurrency;
      while (jsonCurrency.length < 3) {
        jsonCurrency = '0' + jsonCurrency;
      }
      return currentCurrencySign + jsonCurrency.substr(0, jsonCurrency.length - 2) + '.' + jsonCurrency.substr(-2);
    },
    renderJSONCurrencyFromUI: function(uiCurrency) {
      // uiCurrency is a whole unit of currency
      var re = new RegExp('[^\\d' + currencySeparator + '.-]', 'g')
      return parseFloat(uiCurrency.replace(re, '')) * 100;
    },
  }
})()
