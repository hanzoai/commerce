var BuildTable = (function() {
  var tableRowTemplate = '<tr></tr>';
  var tableDataTemplate = '<td></td>';
  var tableHeaderTemplate = '<thead></thead>';
  var tableHeaderDataTemplate = '<th></th>';
  var tableBodyTemplate = '<tbody></tbody>';
  var checkbox = '<label class="csscheckbox csscheckbox-primary"><input type="checkbox"><span></span></label>'

  var currencyCharacters = {
	'usd': '$',
	'aud': '$',
	'cad': '$',
	'eur': '€',
	'gbp': '£'
  };

  return function ($table, tableConfig) {
    // Config is in the form of
    //  {
    //      columns: [{
    //          field: 'field1',
    //          name: 'Field 1',
    //          css: { ... } //optional
    //      },
    //      {
    //          field: 'field2.field3.field4', // chained dot notation supported
    //          name: 'Field 2',
    //          type: "text" // defaults to text
    //      }],
    //      api: 'api.suchtees.com/object',
    //      apiToken: '',
    //      display: [5, 10 , 20]
    //      startPage: 1,
    //      startDisplay: 10,
    //      $pagination: $('#pagination')
    //  }
    //
    // API call is expected to return something in the form of
    //  {
    //      page: 1,
    //      display: 20,
    //      count: 200,
    //      models: [
    //          {...}
    //          {...}
    //      ]
    //  }

    // Build the header
    var $tableHeader = $(tableHeaderTemplate);
    var $tableHeaderCheckbox = $(checkbox);
    var $tableHeaderDataCheckbox = $(tableHeaderDataTemplate).append($tableHeaderCheckbox).css('width', '80px').addClass('text-center');
    var $tableHeaderRow = $(tableRowTemplate).append($tableHeaderDataCheckbox);
    $tableHeader.html($tableHeaderRow);
    $table.append($tableHeader);

    var columns = tableConfig.columns;
    var columnLen = columns.length;
    for (var c = 0; c < columnLen; c++) {
      var column = columns[c];
      var $tableHeaderData = $(tableHeaderDataTemplate).html(column.name);
      if (column.css) {
        $tableHeaderData.css(column.css);
      }
      $tableHeaderRow.append($tableHeaderData);
    }

    // Build the body frame
    var $tableBody = $(tableBodyTemplate);
    $table.append($tableBody);

    // Configure the path vars
    var path = tableConfig.api + '?token=' + tableConfig.apiToken;
    var display = tableConfig.startDisplay; //hard coded for now

    var $pagination = tableConfig.$pagination;
    var ignorePage = false; // set this to prevent infinite looping due to setting max_page

    // Page change callback for filling the body frame
    function paged(page) {
      $.getJSON(path + '&page=' + page + '&display=' + display, function(data){
        $tableBody.html('');

        var maxPage = Math.ceil(data.count/data.display);

        if (maxPage > 1) {
          ignorePage = true;
          $pagination.show()
          $pagination.jqPagination('option', 'max_page', maxPage);
          ignorePage = false;
        } else {
          $pagination.hide()
        }

        // Get the models and start loading rows
        var models = data.models;
        var modelsLen = data.models.length;
        for (var m = 0; m < modelsLen; m++) {

          var model = models[m];

          var $tableCheckbox = $(checkbox);
          var $tableDataCheckbox = $(tableDataTemplate).append($tableCheckbox).addClass('text-center');
          var $tableRow = $(tableRowTemplate).append($tableDataCheckbox);

          for(var c = 0; c < columnLen; c++) {
            var column = columns[c];
            var $tableData = $(tableDataTemplate)

            var type = 'text';
            if (column.type) {
              type = column.type;
            }

            // Handle fields in the form of field1.field2.field3
            var fields = column.field.split('.');
            var fieldsLen = fields.length;

            var val = model;
            for (var f = 0; f < fieldsLen; f++) {
              val = val[fields[f]];
              // deal with the case where the element does not exist
              if (val == null) {
                val = '';
                break;
              }
            }

            // Handle different types of column formatting
            if (type == 'currency' && model.currency) {
              val = currencyCharacters[model.currency] + val;
              val = val.substr(0, val.length - 2) + '.' + val.substr(-2);
              $tableData.addClass('text-right');
            } else if (type == 'date') {
              val = (new Date(val)).toDateString();
            }

            $tableData.html(val);
            $tableRow.append($tableData);
          }

          $tableBody.append($tableRow);
        }
      });
    }

    // Setup pagination
    $pagination.jqPagination({
      paged: function(page) {
        if (!ignorePage) {
          // Clear body frame
          paged(page);
        }
      }
    });

    // Run pagination pass
    paged(tableConfig.startPage);
  }
})()
