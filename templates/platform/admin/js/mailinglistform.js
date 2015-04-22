BuildForm($('#form-mailinglist'),
  [
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
      id: 'id',
      name: 'mailchimp[id]',
      label: 'MailChimp ID',
      type: 'text',
      asterisk: false,
      rules: [
        {
          rule: 'required',
          value: true,
          message: 'Enter your mailing list\'s MailChimp API Key'
        }
      ],
      $parent: $('#mailinglist-info'),
      value: '{{ mailingList.MailchimpList.Id }}',
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
      $parent: $('#mailinglist-info'),
      value: '{{ mailingList.MailchimpList.APIKey }}',
    },
    {
      id: 'doubleOptin',
      name: 'mailchimp[doubleOptin]',
      label: 'Double Opt-in?',
      type: 'switch',
      $parent: $('#mailinglist-mailchimp'),
      labelCols: 6,
      valueCols: 6,
      value: '{{ mailingList.MailchimpList.DoubleOptin }}' === "True",
    },
    {
      id: 'updateExisting',
      name: 'mailchimp[updateExisting]',
      label: 'Update Existing?',
      type: 'switch',
      $parent: $('#mailinglist-mailchimp'),
      labelCols: 6,
      valueCols: 6,
      value: '{{ mailingList.MailchimpList.UpdateExisting }}' === 'True',
    },
    {
      id: 'replaceInterests',
      name: 'mailchimp[replaceInterests]',
      label: 'Replace Interests?',
      type: 'switch',
      $parent: $('#mailinglist-mailchimp'),
      labelCols: 6,
      valueCols: 6,
      value: '{{ mailingList.MailchimpList.ReplaceInterests }}' === 'True',
    },
  ]);
});

