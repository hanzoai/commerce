var Util = (function() {
  var currencySigns = {
    'aud':'$',
	'cad':'$',
	'eur':'€',
	'gbp':'£',
	'hkd':'$',
	'jpy':'¥',
	'nzd':'$',
	'sgd':'$',
	'usd':'$',
	'ghc':'¢',
    'ars':'$',
    'bsd':'$',
    'bbd':'$',
    'bmd':'$',
    'bnd':'$',
    'kyd':'$',
    'clp':'$',
    'cop':'$',
    'xcd':'$',
    'svc':'$',
    'fjd':'$',
    'gyd':'$',
    'lrd':'$',
    'mxn':'$',
    'nad':'$',
    'sbd':'$',
    'srd':'$',
    'tvd':'$',
    'bob':'$b',
    'uyu':'$u',
    'egp':'£',
    'fkp':'£',
    'gip':'£',
    'ggp':'£',
    'imp':'£',
    'jep':'£',
    'lbp':'£',
    'shp':'£',
    'syp':'£',
    'cny':'¥',
    'afn':'؋',
    'thb':'฿',
    'khr':'៛',
    'crc':'₡',
    'trl':'₤',
    'ngn':'₦',
    'kpw':'₩',
    'krw':'₩',
    'ils':'₪',
    'vnd':'₫',
    'lak':'₭',
    'mnt':'₮',
    'cup':'₱',
    'php':'₱',
    'uah':'₴',
    'mur':'₨',
    'npr':'₨',
    'pkr':'₨',
    'scr':'₨',
    'lkr':'₨',
    'irr':'﷼',
    'omr':'﷼',
    'qar':'﷼',
    'sar':'﷼',
    'yer':'﷼',
    'pab':'b/.',
    'vef':'bs',
    'bzd':'bz$',
    'nio':'c$',
    'chf':'chf',
    'huf':'ft',
    'awg':'ƒ',
    'ang':'ƒ',
    'pyg':'gs',
    'jmd':'j$',
    'czk':'kč',
    'bam':'km',
    'hrk':'kn',
    'dkk':'kr',
    'eek':'kr',
    'isk':'kr',
    'nok':'kr',
    'sek':'kr',
    'hnl':'l',
    'ron':'lei',
    'all':'lek',
    'lvl':'ls',
    'ltl':'lt',
    'mzn':'mt',
    'twd':'nt$',
    'bwp':'p',
    'byr':'p.',
    'gtq':'q',
    'zar':'r',
    'brl':'r$',
    'dop':'rd$',
    'myr':'rm',
    'idr':'rp',
    'sos':'s',
    'pen':'s/.',
    'ttd':'tt$',
    'zwd':'z$',
    'pln':'zł',
    'mkd':'ден',
    'rsd':'Дин.',
    'bgn':'лв',
    'kzt':'лв',
    'kgs':'лв',
    'uzs':'лв',
    'azn':'ман',
    'rub':'руб',
    'inr':'',
    'try':'',
	''   : '',
  };

  var currencySeparator = '.';
  var currentCurrencyCode = '';
  var currentCurrencySign = currencySigns[currentCurrencyCode];
  var digitsOnlyRe = new RegExp('[^\\d.-]', 'g')

  var isZeroDecimal = function(code) {
    if (code === 'bif' || code === 'clp' || code === 'djf' || code === 'gnf' || code === 'jpy' || code === 'kmf' || code === 'krw' || code === 'mga' || code === 'pyg' || code === 'rwf' || code === 'vnd' || code === 'vuv' || code === 'xaf' || code === 'xof' || code === 'xpf') {
      return true
    }
    return false
  };

  return {
    setCurrency: function(code){
      currentCurrencyCode = code;
      currentCurrencySign = currencySigns[code];
    },
    renderUpdatedUICurrency: function(uiCurrency) {
      return Util.renderUICurrencyFromJSON(Util.renderJSONCurrencyFromUI(uiCurrency));
    },
    renderUICurrencyFromJSON: function(jsonCurrency) {
      jsonCurrency = '' + jsonCurrency;
      // jsonCurrency is not cents
      if (isZeroDecimal(currentCurrencyCode)) {
        return currentCurrencySign + jsonCurrency
      }

      // jsonCurrency is cents
      while (jsonCurrency.length < 3) {
        jsonCurrency = '0' + jsonCurrency;
      }

      return currentCurrencySign + jsonCurrency.substr(0, jsonCurrency.length - 2) + '.' + jsonCurrency.substr(-2);
    },
    renderJSONCurrencyFromUI: function(uiCurrency) {
      if (isZeroDecimal(currentCurrencyCode)) {
        return parseInt(('' + uiCurrency).replace(digitsOnlyRe, '').replace(currencySeparator, ''), 10)
      }
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
      return parseInt(parseFloat(parts[0].replace(digitsOnlyRe, '')) * 100 + parseFloat(parts[1].replace(digitsOnlyRe, '')), 10);
    },
  }
})()
