func (r *NpmAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	var app v1.NpmApp
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	commit, err := getGitSHA(app)
	if err != nil {
		app.Status.Phase = "Failed"
		_ = r.Status().Update(ctx, &app)
		return ctrl.Result{}, err
	}

	app.Status.Commit = commit

	image := resolveImage(app)

	// BUILD PHASE
	done, err := ensureBuildJob(ctx, r.Client, app, image, commit)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !done {
		app.Status.Phase = "Building"
		_ = r.Status().Update(ctx, &app)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// DEPLOY PHASE
	if err := r.ensureDeployment(ctx, app, image); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureService(ctx, app); err != nil {
		return ctrl.Result{}, err
	}

	app.Status.Phase = "Ready"
	app.Status.Image = image
	app.Status.LastGoodImage = image
	_ = r.Status().Update(ctx, &app)

	return ctrl.Result{}, nil
}