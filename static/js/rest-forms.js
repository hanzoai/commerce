// This function creates handlers for the Post/Get handlers of a restful api
var NewRestAPI = (function() {
  return function(endpoint, token, onErrorHandler, fieldProcessors) {
    if (!fieldProcessors) {
      fieldProcessors = {};
    }

    if (!onErrorHandler) {
      onErrorHandler = function(){};
    }

    return {
      // Get request
      get: function(handler, params) {
        this.ajax('GET', handler, null, null, params);
      },
      // Post request
      post: function(handler, $form, params) {
        this.ajax('POST', handler, $form, null, params);
      },
      // Put request
      put: function(handler, $form, id, params) {
        this.ajax('POST', handler, $form, id, params);
      },
      del: function(handler, id, params) {
        this.ajax('DELETE', handler, null, id, params);
      },
      ajax: function(method, handler, $form, id, params) {
        var paramStrs = [];
        var prop;
        if (params) {
          for (prop in params) {
            if (params.hasOwnProperty(prop)) {
              paramStrs.push(prop + '=' + params[prop]);
            }
          }
        }

        var formJSON;
        if ($form) {
          var formObj = $form.serializeObject();
          for (prop in formObj) {
            if (fieldProcessors[prop]) {
              formObj[prop] = fieldProcessors[prop](formObj[prop]);
            }
          }
          formJSON = JSON.stringify(formObj);
        } else {
          formJSON = '';
        }

        // Construct URI optionally using id
        var uri = (endpoint + '/' + (id || '')).replace(/\/+$/, '');

        // Append params to URI
        if (paramStrs.length > 0) {
          uri = uri + '?' + paramStrs.join('&');
        }

        $.ajax({
          url: uri,
          type: method,
          headers: {Authorization: token},
          data: formJSON,
          contentType: 'application/json; charset=utf-8',
          dataType: 'json',
          success: handler,
          failure: onErrorHandler
        });
      },
    };
  }
})();
