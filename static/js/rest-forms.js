// This function creates handlers for the Post/Get handlers of a restful api
var NewRestAPI = (function() {
  return function(apiUrl, token, onErrorHandler, fieldProcessors) {
    // Make sure there is a slash on the path
    var path = apiUrl;
    if (path.substr(-1) !== '/') {
      path += '/'
    }
    path += '?';

    // Make the token string
    var tokenStr = 'token=' + token;

    if (!onErrorHandler) {
      onErrorHandler = function(){};
    }

    return {
      // Get request
      get: function(handler, params) {
        var paramStrs = [tokenStr];
        if (params) {
          for (var prop in params) {
            if(params.hasOwnProperty(prop)){
              paramStrs.push(prop + '=' + params[prop]);
            }
          }
        }
        $.getJSON(path + paramStrs.join('&'), handler, onErrorHandler)
      },
      // Post request
      post: function(handler, $form, id, params) {
        var paramStrs = [tokenStr];
        if (id == null) {
          id = '';
        }
        if (params) {
          for (var prop in params) {
            if (params.hasOwnProperty(prop)) {
              paramStrs.push(prop + '=' + params[prop]);
            }
          }
        }

        var formJSON;
        if ($form) {
          var formObj = $form.serializeObject();
          for (var prop in formObj) {
            if (fieldProcessors[prop]) {
              formObj[prop] = fieldProcessors[prop](formObj[prop]);
            }
          }
          formJSON = JSON.stringify(formObj);
        } else {
          formJSON = '';
        }

        $.ajax({
          type: 'POST',
          url: path + paramStrs.join('&'),
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
