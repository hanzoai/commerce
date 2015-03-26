var BuildForm = (function() {
  var formGroupInputTemplate = '<div class="form-group"><label class="control-label" for=""></label><div><input type="text" id="" name="" class="form-control" value=""></div></div>';
  var formGroupTextAreaTemplate = '<div class="form-group"><label class="control-label" for=""></label><div><textarea class="form-control" style="resize:none;height:265px"></textarea></div></div>';
  var formGroupLinkTemplate =   '<div class="form-group"><label class="control-label" for=""></label><div><p class="form-control-static"><a></a></p></div></div>';
  var formGroupStaticTemplate = '<div class="form-group"><label class="control-label" for=""></label><div><p class="form-control-static"></p></div></div>';
  var formGroupSelectTemplate = '<div class="form-group"><label class="control-label" for=""></label><div><select class="form-control"></select></div></div>';
  var formGroupSwitchTemplate = '<div class="form-group"><label class="control-label" for=""></label><div><label class="switch switch-success"><input type="checkbox"><span></span></label></div></div>';
  var optionTemplate = '<option></option>';

  var requiredAsteriskTemplate = '<span class="text-danger enable-tooltip" data-original-title="required">*</span>';

  $.validator.addMethod('currency', function(value, element, param) {
      var isParamString = typeof param === 'string',
          symbol = isParamString ? param : param[0],
          soft = isParamString ? true : param[1],
          regex;

      symbol = symbol.replace(/,/g, '');
      symbol = soft ? symbol + ']' : symbol + ']?';
      regex = '^[' + symbol + '([1-9]{1}[0-9]{0,2}(\\,[0-9]{3})*(\\.[0-9]{0,2})?|[1-9]{1}[0-9]{0,}(\\.[0-9]{0,2})?|0(\\.[0-9]{0,2})?|(\\.[0-9]{1,2})?)$';
      regex = new RegExp(regex);
      return this.optional(element) || regex.test(value);

  }, 'Please specify a valid currency');

  return function ($form, inputConfigs) {
    // Config is in the form of
    //  [
    //      {
    //          id: "name",
    //          name: "Model.Name",
    //          label: "Name",
    //          value: "Name",
    //          value: [ // for select
    //              {
    //                  id: "Id",
    //                  name: "Name",
    //                  selected: true,
    //              }
    //          ],
    //          placeholder: "Name",
    //          asterisk: true,
    //          type: "text",
    //          $parent: $('#id'),
    //          labelCols: 3,
    //          valueCols: 9,
    //          href: "lolololol.html, // for link type only
    //          rules: [
    //              {
    //                  rule: "required",
    //                  value: "true",
    //                  message: "this field is required",
    //              }
    //          ]
    //      }
    //  ]
    var ruleSets = {};
    var messages = {};
    var $fgs = [];

    // Convert input configs to validation rules and elements
    var len = inputConfigs.length;
    for (var i = 0; i < len; i++) {
      var inputConfig = inputConfigs[i];

      // Prepare validation logic
      var ruleSet = ruleSets[inputConfig.name] = {};
      var message = messages[inputConfig.name] = {};

      // Trued if we run into a required rule
      var isRequired = false;

      // Make sure rules array exists
      if (inputConfig.rules != null) {
        // Loop over rules
        var rLen = inputConfig.rules.length
        for (var r = 0; r < rLen; r++) {
          var rule = inputConfig.rules[r];

          // True the required rule
          if (rule.rule === 'required') {
            isRequired = true;
          }

          // Do validation logic conversion
          ruleSet[rule.rule] = rule.value;
          message[rule.rule] = rule.message;
        }
      }

      var labelCols = inputConfig.labelCols;
      var valueCols = inputConfig.valueCols;

      var type = inputConfig.type;
      var $fg;
      // Prepare templating
      if (type === 'link') {
        $fg = $(formGroupLinkTemplate);
        $fg.find('a').attr({
          href: inputConfig.href
        }).text(inputConfig.value)
      } else if (type === 'switch') {
        $fg = $(formGroupSwitchTemplate);

        $fg.find('input').attr({
          id: inputConfig.id,
          name: inputConfig.name,
        }).prop('checked', inputConfig.value);

        labelCols = 10;
        valueCols = 2;
      } else if (type === 'select') {
        $fg = $(formGroupSelectTemplate);
        var $select = $fg.find('select').attr({
          id: inputConfig.id,
          name: inputConfig.name,
        });

        var values = inputConfig.value;
        var valueLen = values.length;
        for (var v = 0; v < valueLen; v++) {
          var value = values[v];
          var $option = $(optionTemplate).attr({
            value: value.id,
          }).text(value.name);

          if (value.selected) {
            $option.prop('selected', true);
          }
          $select.append($option);
        }
        $select.chosen({width: '100%', 'disable_search_threshold': 3})
      } else if (type === 'textarea') {
        $fg = $(formGroupTextAreaTemplate);
        $fg.find('textarea').attr({
          id: inputConfig.id,
          name: inputConfig.name,
          placeholder: inputConfig.placeholder,
        }).text(inputConfig.value);
      } else if (type === 'static'){
        $fg = $(formGroupStaticTemplate);
        $fg.find('p').text(inputConfig.value);
      } else if (type === 'static-date'){
        $fg = $(formGroupStaticTemplate);
        var val = (new Date(inputConfig.value)).toDateString();
        $fg.find('p').text(val);
      } else if (type === 'static-currency'){
        $fg = $(formGroupStaticTemplate);
        $fg.find('p').text(Util.renderUICurrencyFromJSON(inputConfig.value)).addClass('text-right')
      } else {
        $fg = $(formGroupInputTemplate);

        var $input = $fg.find('input').attr({
          id: inputConfig.id,
          name: inputConfig.name,
          type: type,
          value: inputConfig.value,
          placeholder: inputConfig.placeholder,
        });

        if (type === 'currency') {
          $input.addClass('text-right');
        }
      }

      // Set label to name
      var label = inputConfig.label;

      // Append asterisk if required
      if (isRequired && inputConfig.asterisk !== false) {
        label += requiredAsteriskTemplate;
      }

      $fg.find('label:first').attr('for', inputConfig.id).html(label).addClass('col-md-' + (labelCols || 3));
      $fg.find('div:first').addClass('col-md-' + (valueCols || 9));

      if (inputConfig.$parent != null) {
        inputConfig.$parent.append($fg);
      } else {
        $fgs.push($fg)
      }
    }

    while ($fgs.length > 0) {
      $form.prepend($fgs.pop());
    }

    $form.validate({
      errorClass: 'help-block animation-slideUp', // You can change the animation class for a different entrance animation - check animations page
      errorElement: 'div',
      errorPlacement: function(error, e) {
        e.parents('.form-group > div').append(error);
      },
      highlight: function(e) {
        $(e).closest('.form-group').removeClass('has-success has-error').addClass('has-error');
        $(e).closest('.help-block').remove();
      },
      success: function(e) {
        e.closest('.form-group').removeClass('has-success has-error');
        e.closest('.help-block').remove();
      },
      rules: ruleSets,
      messages: messages,
    })
  };
})();
