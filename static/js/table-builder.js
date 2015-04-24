var BuildTable = (function() {
  var tableRowTemplate = '<tr></tr>';
  var tableDataTemplate = '<td></td>';
  var tableHeaderTemplate = '<thead></thead>';
  var tableHeaderDataTemplate = '<th style="white-space:nowrap;cursor:pointer;"></th>';
  var tableBodyTemplate = '<tbody></tbody>';
  var checkboxTemplate = '<label class="csscheckbox csscheckbox-primary"><input type="checkbox"><span></span></label>';
  var selectTemplate = '<select class="form-control"></select>';
  var optionTemplate = '<option value=""></option>';
  var displayLabelTemplate = '<label>&nbsp;&nbsp;Items per Page</label>';
  var deleteTemplate = '<a class="btn btn-xs btn-danger">x</a>';

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
    //      $empty: $('#empty'),
    //      canDelete: true,
    //      href: "lolololol.html", // for render link
    //      checkboxes: false, //defaults to true
    //      data: [{
    //          field: 'data to load',
    //        },
    //        {
    //          field: 'if no api url',
    //        }]
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
    var lastPage = tableConfig.startPage;
    var sort = '-UpdatedAt';
    var $lastSort = $();
    var lastSortName = '';

    var $empty = tableConfig.$empty;

    // Build the header
    var $tableHeader = $(tableHeaderTemplate);
    var $tableHeaderRow = $(tableRowTemplate);

    if (tableConfig.checkboxes !== false) {
      var $tableHeaderCheckbox = $(checkboxTemplate);

      $tableHeaderCheckbox.find(':checkbox').on('change', function() {
        var checkedStatus   = $(this).prop('checked');
        $table.find(':checkbox').prop('checked', checkedStatus);
      });

      var $tableHeaderDataCheckbox = $(tableHeaderDataTemplate).append($tableHeaderCheckbox).css('width', '80px').addClass('text-center');
      $tableHeaderRow.append($tableHeaderDataCheckbox);
    }

    $tableHeader.html($tableHeaderRow);
    $table.append($tableHeader);

    var columns = tableConfig.columns;
    var columnLen = columns.length;
    for (var c = 0; c < columnLen; c++) {
      var column = columns[c];
      (function(column) {
        var $tableHeaderData = $(tableHeaderDataTemplate).html(column.name);
        if (column.css) {
          $tableHeaderData.css(column.css);
        }
        $tableHeaderRow.append($tableHeaderData);

        $tableHeaderData.on('click', function(){
          var newSort = column.field;
          if (newSort == 'id') {
            newSort = 'Id_'
          } else {
            newSort = newSort.charAt(0).toUpperCase() + newSort.slice(1);
          }

          var direction = 'sort-asc'
          var i = sort.indexOf(newSort);
          if (i === 0) {
            sort = '-' + newSort;
            var direction = 'sort-desc'
          } else {
            sort = newSort;
          }

          $lastSort.html(lastSortName)
          $lastSort = $tableHeaderData;
          lastSortName = column.name;

          $tableHeaderData.html(column.name + '&nbsp;<i class="fa fa-' + direction + '"></i>')
          paged(lastPage);
        });
      })(column)
    }

    if (tableConfig.canDelete) {
      var $tableHeaderDataDelete = $(tableHeaderDataTemplate).append('Delete').css('width', '80px')
      $tableHeaderRow.append($tableHeaderDataDelete);
    }

    // Build the body frame
    var $tableBody = $(tableBodyTemplate);
    $table.append($tableBody);

    // Configure the path vars
    var path = tableConfig.apiUrl
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

    // Chosen Select UI garish
    $tableDisplaySelect.chosen({width: '60px', 'disable_search_threshold': 3})

    var $pagination = tableConfig.$pagination;
    var ignorePage = false; // set this to prevent infinite looping due to setting max_page

    function load(data) {
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
        var $tableRow = $(tableRowTemplate);

        if (tableConfig.checkboxes !== false) {
          var $tableCheckbox = $(checkboxTemplate);
          var $tableDataCheckbox = $(tableDataTemplate).append($tableCheckbox).addClass('text-center');
          $tableRow.append($tableDataCheckbox);
        }

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
          if (typeof render === 'function') {
            val = render(val, model, $tableData);
          } else if (render == 'text') {
            val = "" + val;
            val = val.charAt(0).toUpperCase() + val.slice(1);
          } else if (render == 'currency' && model.currency) {
            Util.setCurrency(model.currency);
            val = Util.renderUICurrencyFromJSON(val);
            $tableData.addClass('text-right');
          } else if (render == 'date') {
            val = DateFormat.format.date(val, 'yyyy-MM-dd hh:mm');
            $tableData.addClass('text-center');
          } else if (render == 'ago') {
            val = $.timeago(val);
            $tableData.addClass('text-center');
          } else if (render == 'number') {
            $tableData.addClass('text-right');
          } else if (render == 'id') {
            val = $('<a href="' + tableConfig.itemUrl + '/' + val + '">' + val + '</a>')
          } else if (render == 'link') {
            val = $('<a href="' + column.href + '/' + val + '">' + val + '</a>')
          } else if (render == 'upper') {
            val = val.toUpperCase();
          } else if (render == 'bool') {
            val = val ? 'Yes' : 'No';
          } else if (render == 'snippet' && model.id){
            val = $('<textarea class="form-control" style="height:80px;"><script src="' + column.apiUrl + 'mailinglist/' + model.id + '/js"></script></textarea>');
          } else if (render && {}.toString.call(render) == '[object Function]') {
            val = render(val, model);
          }

          $tableData.html(val);
          $tableRow.append($tableData);
        }

        $tableBody.append($tableRow);

        if (tableConfig.canDelete) {
          var $dataDelete = $(deleteTemplate)
          bindDeleteRow($dataDelete, tableConfig.apiUrl, model.id);

          var $tableDataDelete = $(tableDataTemplate)
            .append($dataDelete).css('width', '80px')
            .addClass('text-center');
          $tableRow.append($tableDataDelete);
        }
      }
    }

    // if no api data is present, then load and then quit
    if (tableConfig.apiUrl === '' || tableConfig.apiUrl == null) {
      $tableDisplay.hide();
      var fakeData = {
        page: 1,
        display: 1000,
        count: 1000,
        models: tableConfig.data || [],
      }
      load(fakeData);
      return;
    }

    // Page change callback for filling the body frame
    function paged(page) {
      $.ajax({
        type: 'GET',
        headers: {Authorization: tableConfig.apiToken},
        url: path + '?page=' + page + '&display=' + display + '&sort=' + sort,
        success: function(data){
          load(data);
        }
      });
    }

    function bindDeleteRow($el, apiUrl, id) {
      $el.on('click', function() { deleteRow(apiUrl + '/' + id); });
    }

    function deleteRow(path) {
      $.ajax({
        type: 'DELETE',
        headers: {Authorization: tableConfig.apiToken},
        url: path,
        success: function () {
          paged(lastPage);
        }
      })
    }

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
