#!/usr/bin/python

# General libraries
import json
import logging
import sys
import traceback

# App engine specific libraries
from google.appengine.api import channel
from google.appengine.api import users
from google.appengine.ext.webapp.util import login_required
import webapp2
from webapp2_extras import jinja2


class UpdateMetadata(webapp2.RequestHandler):
    # Can't use login_required decorator here because it is not supported for
    # POST requests

    def is_loggedin(self):
        try:
            users.get_current_user().user_id()
            return True
        except:
            logging.error(traceback.format_exception(*sys.exc_info()))
            return False

    def post(self):
        response = {'succeeded': False}
        if not self.is_loggedin():
            return self.response.write(json.dumps(response))

        response = {'succeeded': True}
        self.response.write(json.dumps(response))


class Index(webapp2.RequestHandler):
    """Handler to serve main page with static content."""

    def render(self, template, **context):
        """Use Jinja2 instance to render template and write to output.

        Args:
          template: filename (relative to $PROJECT/templates) that we are rendering
          context: keyword arguments corresponding to variables in template
        """
        jinja2_renderer = jinja2.get_jinja2(app=self.app)
        rendered_value = jinja2_renderer.render_template(template, **context)
        self.response.write(rendered_value)

    @login_required
    def get(self):
        user_id = users.get_current_user().user_id()
        channel.create_channel(user_id)  # login?
        self.response.write("ok")


app = webapp2.WSGIApplication([
    ('/update-metadata', UpdateMetadata),
    ('/', Index)
], debug=True)
