var BuildTable = (function() {
  var tableRowTemplate = '<tr></tr>';
  var tableDataTemplate = '<td></td>';
  var tableHeaderTemplate = '<thead></thead>';
  var tableHeaderDataTemplate = '<th></th>';
  var tableBodyTemplate = '<tbody></tbody>';
  var checkboxTemplate = '<label class="csscheckbox csscheckbox-primary"><input type="checkbox"><span></span></label>';
  var selectTemplate = '<select class="form-control"></select>';
  var optionTemplate = '<option value=""></option>';
  var displayLabelTemplate = '<label>&nbsp;&nbsp;Items per Page</label>';

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
    //          render: "text" // defaults to text
    //      }],
    //      itemUrl: 'admin.suchtees.com/object',
    //      apiUrl: 'api.suchtees.com/object',
    //      apiToken: '',
    //      displayOptions: [5, 10 , 20]
    //      startPage: 1,
    //      startDisplay: 10,
    //      $display: $('#display'),
    //      $pagination: $('#pagination'),
    //      $empty: $('#empty')
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
    var $empty = tableConfig.$empty;

    // Build the header
    var $tableHeader = $(tableHeaderTemplate);
    var $tableHeaderCheckbox = $(checkboxTemplate);

    $tableHeaderCheckbox.find(':checkbox').on('change', function() {
      var checkedStatus   = $(this).prop('checked');
      $table.find(':checkbox').prop('checked', checkedStatus);
    });

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
    var tokenStr = '?token=' + tableConfig.apiToken;
    var path = tableConfig.apiUrl + tokenStr;
    var display = tableConfig.startDisplay; //hard coded for now

    // Build the display options
    var $tableDisplaySelect = $(selectTemplate);
    var $tableDisplayLabel = $(displayLabelTemplate);

    var $tableDisplay = tableConfig.$display;
    $tableDisplay.append($tableDisplaySelect);
    $tableDisplay.append($tableDisplayLabel);

    var displayOptions = tableConfig.displayOptions;
    var displayOptionsLen = displayOptions.length;
    for (var d = 0; d < displayOptionsLen; d++) {
      var displayOption = displayOptions[d];
      var $tableDisplayOption = $(optionTemplate).attr('value', displayOption).html(displayOption);
      if (displayOption === display) {
        $tableDisplayOption.attr('selected', 'selected');
      }
      $tableDisplaySelect.append($tableDisplayOption);
    }

    var $pagination = tableConfig.$pagination;
    var ignorePage = false; // set this to prevent infinite looping due to setting max_page

    // Page change callback for filling the body frame
    function paged(page) {
      $.getJSON(path + '&page=' + page + '&display=' + display, function(data){
        if (data.count === 0) {
          $empty.show();
          $table.closest('.block').hide();
        }
        // Clear body frame
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

          var $tableCheckbox = $(checkboxTemplate);
          var $tableDataCheckbox = $(tableDataTemplate).append($tableCheckbox).addClass('text-center');
          var $tableRow = $(tableRowTemplate).append($tableDataCheckbox);

          for(var c = 0; c < columnLen; c++) {
            var column = columns[c];
            var $tableData = $(tableDataTemplate)

            var render = 'text';
            if (column.render) {
              render = column.render;
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

            // Handle different renders of column formatting
            if (render == 'text') {
              val = val.charAt(0).toUpperCase() + val.slice(1);
            } else if (render == 'currency' && model.currency) {
              val = currencyCharacters[model.currency] + val;
              val = val.substr(0, val.length - 2) + '.' + val.substr(-2);
              $tableData.addClass('text-right');
            } else if (render == 'date') {
              val = (new Date(val)).toDateString();
            } else if (render == 'number') {
              $tableData.addClass('text-right');
            } else if (render == 'id') {
              $tableData.addClass('text-center');
              val = $('<a href="' + tableConfig.itemUrl + '/' + val + '">' + val + '</a>')
            } else if (render == 'bool') {
              val = val ? 'Yes' : 'No';
            } else if (render && {}.toString.call(render) == '[object Function]') {
              val = render(val, model);
            }

            $tableData.html(val);
            $tableRow.append($tableData);
          }

          $tableBody.append($tableRow);
        }
      });
    }

    var lastPage = tableConfig.startPage;

    // Setup pagination
    $pagination.jqPagination({
      paged: function(page) {
        if (!ignorePage) {
          lastPage = page;
          paged(page);
        }
      }
    });

    // Setup display option changes
    $tableDisplaySelect.on('change', function() {
      display = parseInt($tableDisplaySelect.val(), 10);
      paged(lastPage);
    });

    // Run pagination pass
    paged(lastPage);
  }
})()
