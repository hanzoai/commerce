var BuildTable = (function() {
  var tableRowTemplate = '<tr></tr>';
  var tableDataTemplate = '<td></td>';
  var tableHeaderTemplate = '<thead></thead>';
  var tableHeaderDataTemplate = '<th></th>';
  var tableBodyTemplate = '<tbody></tbody>';
  var checkbox = '<label class="csscheckbox csscheckbox-primary"><input type="checkbox"><span></span></label>'

  return function ($tableEl, tableConfig) {
    // Config is in the form of
    //  {
    //      columns: [{
    //          field: 'field1',
    //          name: 'Field 1'
    //      },
    //      {
    //          field: 'field2',
    //          name: 'Field 2'
    //      }],
    //      api: 'api.suchtees.com/object',
    //      apiToken: '',
    //      display: [5, 10 , 20]
    //      startPage: 1,
    //      startDisplay: 10,
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
    var $tableHeaderDataCheckbox = $(tableHeaderDataTemplate).append($tableHeaderCheckbox);
    var $tableHeaderRow = $(tableRowTemplate).append($tableHeaderDataCheckbox);
    $tableHeader.html($tableHeaderRow);
    $tableEl.append($tableHeader);

    var columns = tableConfig.columns;
    var columnLen = columns.length;
    for (var c = 0; c < columnLen; c++) {
      var $tableHeaderData = $(tableHeaderDataTemplate).html(columns[c].name);
      $tableHeaderRow.append($tableHeaderData);
    }

    // Build the body frame
    var $tableBody = $(tableBodyTemplate);
    $tableEl.append($tableBody);

    // Configure the path vars
    var path = tableConfig.api + '?token=' + tableConfig.apiToken;

    // Page change callback for filling the body frame
    function paged(page, display) {
      $.getJSON(path + '&page=' + page + '&display=' + display, function(data){
        // Clear body frame
        $tableBody.html();

        // Get the models and start loading rows
        var models = data.models;
        var modelsLen = data.models.length;
        for (var m = 0; m < modelsLen; m++) {
          var model = models[m];

          var $tableCheckbox = $(checkbox);
          var $tableDataCheckbox = $(tableDataTemplate).append($tableCheckbox);
          var $tableRow = $(tableRowTemplate).append($tableDataCheckbox);

          for(var c = 0; c < columnLen; c++) {
            var $tableData = $(tableDataTemplate).html(model[columns[c].field]);
            $tableRow.append($tableData);
          }

          $tableBody.append($tableRow);
        }
      });
    }

    paged(tableConfig.startPage, tableConfig.startDisplay);

    return paged;
  }
})()
