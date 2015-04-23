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
      label: 'Id',
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
      id: 'thankYou',
      name: 'thankYou',
      label: 'Thank You URL',
      type: 'text',
      asterisk: false,
      rules: [
        {
          rule: 'required',
          value: true,
          message: 'Enter your mailing list thank you page.'
        }
      ],
      $parent: $('#mailinglist-info'),
      value: '{{ mailingList.ThankYou }}',
    },
    {
      id: 'id',
      name: 'mailchimp[id]',
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
      $parent: $('#mailinglist-mailchimp'),
      value: '{{ mailingList.Mailchimp.Id }}',
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
      $parent: $('#mailinglist-mailchimp'),
      value: '{{ mailingList.Mailchimp.APIKey }}',
    },
    {
      id: 'doubleOptin',
      name: 'mailchimp[doubleOptin]',
      label: 'Double Opt-in?',
      type: 'switch',
      $parent: $('#mailinglist-mailchimp'),
      labelCols: 6,
      valueCols: 6,
      value: '{{ mailingList.Mailchimp.DoubleOptin }}' === 'True'
    },
    {
      id: 'updateExisting',
      name: 'mailchimp[updateExisting]',
      label: 'Update Existing?',
      type: 'switch',
      $parent: $('#mailinglist-mailchimp'),
      labelCols: 6,
      valueCols: 6,
      value: '{{ mailingList.Mailchimp.UpdateExisting }}' === 'True'
    },
    {
      id: 'replaceInterests',
      name: 'mailchimp[replaceInterests]',
      label: 'Replace Interests?',
      type: 'switch',
      $parent: $('#mailinglist-mailchimp'),
      labelCols: 6,
      valueCols: 6,
      value: '{{ mailingList.Mailchimp.ReplaceInterests }}' === 'True'
    },
  ]);
});

