var Util = (function() {
  var currencySigns = {
	'usd': '$',
	'aud': '$',
	'cad': '$',
	'eur': '€',
	'gbp': '£',
	'hkd': '$',
	'jpy': '¥',
	'nzd': '$',
  };

  var currencySeparator = '.';
  var currentCurrencyCode = '';
  var currentCurrencySign = currencySigns[currentCurrencyCode];

  return {
    setCurrency: function(code){
      currentCurrencyCode = code;
      currentCurrencySign = currencySigns[code];
    },
    renderUpdatedUICurrency: function(uiCurrency) {
      return Util.renderUICurrencyFromJSON(Util.renderJSONCurrencyFromUI(uiCurrency));
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
      var parts = uiCurrency.split(currencySeparator);
      if (parts.length > 1) {
        parts[1] = parts[1].substr(0, 2);
        while(parts[1].length < 2) {
          parts[1] += '0';
        }
      } else {
        parts[1] = '00';
      }
      var re = new RegExp('[^\\d.-]', 'g')
      return parseInt(parseFloat(parts[0].replace(re, '')) * 100 + parseFloat(parts[1].replace(re, '')), 10);
    },
  }
})()
