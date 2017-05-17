BuildForm($('#form-mailinglist'),
  [
    {
      id: 'gaCategory',
      name: 'google[category]',
      label: 'Category',
      type: 'text',
      $parent: $('#mailinglist-ga'),
      value: '{{ mailingList.Google.Category }}',
    },
    {
      id: 'gaName',
      name: 'google[name]',
      label: 'Name',
      type: 'text',
      $parent: $('#mailinglist-ga'),
      value: '{{ mailingList.Google.Name }}',
    },
    {
      id: 'fbId',
      name: 'facebook[id]',
      label: 'Id',
      type: 'text',
      $parent: $('#mailinglist-fb'),
      value: '{{ mailingList.Facebook.Id }}',
    },
    {
      id: 'fbValue',
      name: 'facebook[value]',
      label: 'Value',
      type: 'text',
      $parent: $('#mailinglist-fb'),
      value: '{{ mailingList.Facebook.Value }}' || '0.0',
    },
    {
      id: 'fbId',
      name: 'facebook[currency]',
      label: 'Currency',
      type: 'text',
      $parent: $('#mailinglist-fb'),
      value: '{{ mailingList.Facebook.Currency }}' || 'USD',
    },
    {
      id: 'name',
      name: 'name',
      label: 'Name',
      type: 'text',
      asterisk: false,
      rules: [
        {
          rule: 'required',
          value: true,
          message: 'Enter a name for your mailing list'
        }
      ],
      $parent: $('#mailinglist-info'),
      value: '{{ mailingList.Name }}',
    },
    {
      id: 'thankyou-type',
      name: 'thankyou[type]',
      label: 'Thank You Type',
      value: [
        {% for type in constants.ThankYouTypes %}
        {
          selected: '{{ type }}' === '{{ mailingList.ThankYou.Type }}',
          name: '{{ type }}',
          id: '{{ type }}',
        },
        {% endfor %}
      ],
      type: 'select',
      $parent: $('#mailinglist-info'),
    },
    {
      id: 'thankyou',
      label: 'Thank You (URL, HTML, or Javascript)',
      type: 'textarea',
      $parent: $('#mailinglist-info'),
      {% if mailingList.ThankYou.Type == constants.ThankYouTypes.1 %}
      name: 'thankyou[url]',
      value: '{{ mailingList.ThankYou.Url }}',
      {% else %}
      name: 'thankyou[html]',
      value: '{{ mailingList.ThankYou.HTML | escapejs | safe }}',
      {% endif %}
      height: '192px',
    },
    {
      id: 'listId',
      name: 'mailchimp[listId]',
      label: 'MailChimp List ID',
      type: 'text',
      asterisk: false,
      rules: [
        {
          rule: 'required',
          value: true,
          message: 'Enter your mailing list\'s MailChimp API Key'
        }
      ],
      $parent: $('.mailinglist-mailchimp-col1'),
      value: '{{ mailingList.Mailchimp.ListId }}',
    },
    {
      id: 'apiKey',
      name: 'mailchimp[apiKey]',
      label: 'MailChimp API Key',
      type: 'text',
      asterisk: false,
      rules: [
        {
          rule: 'required',
          value: true,
          message: 'Enter your mailing list\'s MailChimp API Key'
        }
      ],
      $parent: $('.mailinglist-mailchimp-col1'),
      value: '{{ mailingList.Mailchimp.APIKey }}',
    },
    {
      id: 'doubleOptin',
      name: 'mailchimp[doubleOptin]',
      label: 'Double Opt-in?',
      type: 'switch',
      $parent: $('.mailinglist-mailchimp-col2'),
      labelCols: 6,
      valueCols: 6,
      value: '{{ mailingList.Mailchimp.DoubleOptin }}' === 'True'
    },
    {
      id: 'updateExisting',
      name: 'mailchimp[updateExisting]',
      label: 'Update Existing?',
      type: 'switch',
      $parent: $('.mailinglist-mailchimp-col2'),
      labelCols: 6,
      valueCols: 6,
      value: '{{ mailingList.Mailchimp.UpdateExisting }}' === 'True'
    },
    {
      id: 'replaceInterests',
      name: 'mailchimp[replaceInterests]',
      label: 'Replace Interests?',
      type: 'switch',
      $parent: $('.mailinglist-mailchimp-col2'),
      labelCols: 6,
      valueCols: 6,
      value: '{{ mailingList.Mailchimp.ReplaceInterests }}' === 'True'
    },
    {
      id: 'sendWelcome',
      name: 'mailchimp[sendWelcome]',
      label: 'Send a Welcome Email?',
      type: 'switch',
      $parent: $('.mailinglist-mailchimp-col2'),
      labelCols: 6,
      valueCols: 6,
      value: '{{ mailingList.Mailchimp.SendWelcome }}' === 'True'
    },
  ]);
});

$(function(){
  $('#thankyou-type').chosen().on('change', function() {
    var val = $(this).val();
    var key = 'html';
    if (val === 'redirect') {
      key = 'url';
    }
    $('#thankyou').attr('name', 'thankyou[' + key + ']');
  });
});

