//go:build linux && webkit2_41

// zoom_linux.go sets WebKit's own page zoom on the window, like a browser's
// Ctrl +/−. CSS zoom on the page confuses menu and tooltip positioning in
// WebKitGTK; real page zoom keeps layout, events and popups consistent.
package main

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1
#include <stdlib.h>
#include <gtk/gtk.h>
#include <webkit2/webkit2.h>

static WebKitWebView *findWebView(GtkWidget *w) {
	if (WEBKIT_IS_WEB_VIEW(w)) return WEBKIT_WEB_VIEW(w);
	if (!GTK_IS_CONTAINER(w)) return NULL;
	WebKitWebView *found = NULL;
	GList *children = gtk_container_get_children(GTK_CONTAINER(w));
	for (GList *l = children; l && !found; l = l->next) found = findWebView(GTK_WIDGET(l->data));
	g_list_free(children);
	return found;
}

static gboolean applyZoom(gpointer data) {
	double z = *(double *)data;
	free(data);
	GList *tops = gtk_window_list_toplevels();
	for (GList *l = tops; l; l = l->next) {
		WebKitWebView *v = findWebView(GTK_WIDGET(l->data));
		if (v) webkit_web_view_set_zoom_level(v, z);
	}
	g_list_free(tops);
	return G_SOURCE_REMOVE;
}

// GTK is only touched on its own thread.
static void scheduleZoom(double z) {
	double *d = malloc(sizeof(double));
	*d = z;
	g_idle_add(applyZoom, d);
}
*/
import "C"

// setNativeZoom zooms the page; true when the platform did it.
func setNativeZoom(z float64) bool {
	C.scheduleZoom(C.double(z))
	return true
}
